package migrations

import (
	"context"
	"fmt"
	"testing"

	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// auditNamedTablesOutsideAuditSchema lists every table outside the audit
// schema whose name says it is an audit trail (#6844). The audit schema has
// its own guard in TestAuditSchemaAppendOnlyForTenantRole; the default ACL of
// the other schemas hands phoenix_tenant UPDATE and DELETE on every new table,
// so only this guard keeps an audit table there append-only.
func auditNamedTablesOutsideAuditSchema(t *testing.T, db *testpkg.DB) []string {
	t.Helper()
	var tables []string
	require.NoError(t, db.NewRaw(`
		SELECT n.nspname || '.' || c.relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind IN ('r', 'p')
		  AND n.nspname NOT IN ('audit', 'pg_catalog', 'information_schema')
		  AND c.relname LIKE '%audit%'
		ORDER BY 1
	`).Scan(context.Background(), &tables))
	return tables
}

// TestAuditTablesAppendOnlyForTenantRole: an audit trail outside the audit
// schema is append-only for the tenant role as well. Expired rows leave only
// through the fixed retention functions, never through a direct DELETE.
func TestAuditTablesAppendOnlyForTenantRole(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	tables := auditNamedTablesOutsideAuditSchema(t, db)
	require.Contains(t, tables, "config.setting_audit", "the guard must see the audit tables outside the audit schema")
	require.Contains(t, tables, "active.staff_absence_audit")

	for _, relation := range tables {
		for _, privilege := range []string{"UPDATE", "DELETE", "TRUNCATE"} {
			assert.Falsef(t, tenantHasPrivilege(t, db, relation, privilege),
				"%s: phoenix_tenant must not hold %s on an audit table. Retention runs through a "+
					"SECURITY DEFINER function with a fixed window; ON DELETE CASCADE needs no privilege.",
				relation, privilege)
		}
	}
}

// TestAuditRetentionFunctionsKeepTheWindow: the retention functions delete
// only expired rows of the caller's own school or Träger; a direct DELETE is
// refused.
func TestAuditRetentionFunctionsKeepTheWindow(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	suffix := testpkg.UniqueTestTenantID(t)
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger Löschfrist", Slug: fmt.Sprintf("traeger-loeschfrist-%d", suffix), Active: true}).ID
	otherOrganization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger Fremd", Slug: fmt.Sprintf("traeger-loeschfrist-fremd-%d", suffix), Active: true}).ID
	school := testpkg.CreateTestSchoolInOrganization(t, db, organization, "Schule Löschfrist")
	otherSchool := testpkg.CreateTestSchoolInOrganization(t, db, otherOrganization, "Schule Fremd")

	const key = "test.audit_retention_6844"
	for _, tenantID := range []int64{school, otherSchool} {
		_, err := db.ExecContext(ctx, `INSERT INTO config.setting_audit (tenant_id, setting_key, action, changed_at)
			VALUES (?, ?, 'set', now() - INTERVAL '3 years 1 day'), (?, ?, 'set', now() - INTERVAL '2 years')`,
			tenantID, key, tenantID, key)
		require.NoError(t, err)
	}
	for _, organizationID := range []int64{organization, otherOrganization} {
		_, err := db.ExecContext(ctx, `INSERT INTO config.traeger_setting_audit (organization_id, setting_key, action, changed_at)
			VALUES (?, ?, 'set', now() - INTERVAL '3 years 1 day'), (?, ?, 'set', now() - INTERVAL '2 years')`,
			organizationID, key, organizationID, key)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, `INSERT INTO platform.traeger_master_data_audit (organization_id, subject, action, occurred_at)
			VALUES (?, 'master_data', 'read', now() - INTERVAL '3 years 1 day'), (?, 'master_data', 'read', now() - INTERVAL '2 years')`,
			organizationID, organizationID)
		require.NoError(t, err)
	}

	for _, ledger := range []struct {
		name, setting, function, table, scope string
		own, other                            int64
	}{
		{"school setting changes", "app.current_tenant_id", "config.delete_expired_setting_audit", "config.setting_audit", "tenant_id", school, otherSchool},
		{"Träger setting changes", "app.current_org_id", "config.delete_expired_traeger_setting_audit", "config.traeger_setting_audit", "organization_id", organization, otherOrganization},
		{"Träger master data log", "app.current_org_id", "platform.delete_expired_traeger_master_data_audit", "platform.traeger_master_data_audit", "organization_id", organization, otherOrganization},
	} {
		t.Run(ledger.name, func(t *testing.T) {
			asRole(t, db, ledger.setting, ledger.own, func(tx bun.Tx) {
				savepoint := func(name string) {
					_, err := tx.ExecContext(ctx, "SAVEPOINT "+name)
					require.NoError(t, err)
				}
				rollbackTo := func(name string) {
					_, err := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT "+name)
					require.NoError(t, err)
				}

				savepoint("direct_delete")
				_, err := tx.ExecContext(ctx, `DELETE FROM `+ledger.table+` WHERE `+ledger.scope+` = ?`, ledger.own)
				assert.Equal(t, "42501", testpkg.PostgresSQLState(err), "the tenant role deletes audit rows only through the retention function")
				rollbackTo("direct_delete")

				savepoint("direct_update")
				_, err = tx.ExecContext(ctx, `UPDATE `+ledger.table+` SET `+ledger.scope+` = `+ledger.scope+` WHERE `+ledger.scope+` = ?`, ledger.own)
				assert.Equal(t, "42501", testpkg.PostgresSQLState(err), "the tenant role changes no audit row")
				rollbackTo("direct_update")

				var deleted int64
				savepoint("foreign_scope")
				err = tx.NewRaw(`SELECT `+ledger.function+`(?)`, ledger.other).Scan(ctx, &deleted)
				assert.Equal(t, "42501", testpkg.PostgresSQLState(err), "the caller cannot delete the audit rows of another scope")
				rollbackTo("foreign_scope")

				require.NoError(t, tx.NewRaw(`SELECT `+ledger.function+`(?)`, ledger.own).Scan(ctx, &deleted))
				assert.EqualValues(t, 1, deleted, "only the expired row goes")

				_, err = tx.ExecContext(ctx, "RESET ROLE")
				require.NoError(t, err)
				var remaining []int64
				require.NoError(t, tx.NewRaw(`SELECT count(*) FROM `+ledger.table+` WHERE `+ledger.scope+` IN (?, ?) GROUP BY `+ledger.scope+` ORDER BY `+ledger.scope+` = ? DESC`,
					ledger.own, ledger.other, ledger.own).Scan(ctx, &remaining))
				assert.Equal(t, []int64{1, 2}, remaining, "the young row stays, the other scope keeps both")
			})
		})
	}
}
