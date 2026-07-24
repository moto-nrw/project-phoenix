package migrations

import (
	"context"
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// auditDeleteAllowlist names the audit tables phoenix_tenant may still DELETE
// from, because their GDPR retention job runs per tenant inside a
// WithTenantTx (SET LOCAL ROLE phoenix_tenant):
//
//   - deviation_events:       TimetableCleanupService.CleanupExpiredTimetableData
//   - unregistered_tag_scans: UnregisteredTagScanService.DeleteOlderThan
//
// Shrink-only. Adding an entry means a new table is no longer append-only —
// justify it in the PR description or find another way to expire the rows
// (an ON DELETE CASCADE from the owning table needs no privilege at all).
var auditDeleteAllowlist = map[string]string{
	"deviation_events":       "timetable GDPR retention runs under phoenix_tenant",
	"unregistered_tag_scans": "90-day scan retention runs under phoenix_tenant",
}

// auditUpdateAllowlist is intentionally EMPTY: no application path modifies an
// existing audit row under the tenant role. ON DELETE SET NULL columns
// (deviation_events.instance_id, data_access_log.student_id,
// unregistered_tag_scans.device_id, …) do not need UPDATE — referential
// actions run with the table owner's privileges.
var auditUpdateAllowlist = map[string]string{}

func auditSchemaTables(t *testing.T, db *bun.DB) []string {
	t.Helper()
	var tables []string
	require.NoError(t, db.NewRaw(`
		SELECT c.relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'audit' AND c.relkind = 'r'
		ORDER BY c.relname
	`).Scan(context.Background(), &tables))
	return tables
}

func tenantHasPrivilege(t *testing.T, db *bun.DB, relation, privilege string) bool {
	t.Helper()
	var granted bool
	require.NoError(t, db.NewRaw(`SELECT has_table_privilege('phoenix_tenant', ?, ?)`, relation, privilege).
		Scan(context.Background(), &granted))
	return granted
}

// TestAuditSchemaAppendOnlyForTenantRole is the ratchet for issue #1924. The
// audit schema must stay append-only for phoenix_tenant: never UPDATE, and
// DELETE only where a tenant-scoped retention job needs it.
func TestAuditSchemaAppendOnlyForTenantRole(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	tables := auditSchemaTables(t, db)
	require.NotEmpty(t, tables, "expected tables in the audit schema")

	for _, table := range tables {
		relation := "audit." + table

		if _, allowed := auditUpdateAllowlist[table]; !allowed {
			assert.Falsef(t, tenantHasPrivilege(t, db, relation, "UPDATE"),
				"%s: phoenix_tenant must not hold UPDATE — audit rows are append-only. "+
					"A new table inherits this from the schema default ACL only if migration "+
					"1.15.225's ALTER DEFAULT PRIVILEGES was undone; otherwise some migration "+
					"granted it explicitly.", relation)
		}

		if _, allowed := auditDeleteAllowlist[table]; !allowed {
			assert.Falsef(t, tenantHasPrivilege(t, db, relation, "DELETE"),
				"%s: phoenix_tenant must not hold DELETE. If a retention job needs it, add the "+
					"table to auditDeleteAllowlist with a reason; if rows expire through an "+
					"ON DELETE CASCADE, no privilege is required.", relation)
		}
	}

	// The one-off migration backups carry no RLS, so any tenant-role access
	// would cross tenant borders. Restores run as phoenix_admin/superuser.
	for _, table := range []string{"room_color_migration_backup", "wc_alias_migration_backup"} {
		relation := "audit." + table
		for _, privilege := range []string{"SELECT", "INSERT", "UPDATE", "DELETE"} {
			assert.Falsef(t, tenantHasPrivilege(t, db, relation, privilege),
				"%s: phoenix_tenant must hold no %s — the table has no RLS policy", relation, privilege)
		}
	}
}

// TestAuditSchemaDefaultACLIsAppendOnly guards the root cause: without this,
// the next audit table silently inherits UPDATE/DELETE again and only the
// per-table assertions above would catch it — after the fact.
func TestAuditSchemaDefaultACLIsAppendOnly(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	var aclItems []string
	require.NoError(t, db.NewRaw(`
		SELECT unnest(d.defaclacl)::text
		FROM pg_default_acl d
		JOIN pg_namespace n ON n.oid = d.defaclnamespace
		WHERE n.nspname = 'audit' AND d.defaclobjtype = 'r'
	`).Scan(context.Background(), &aclItems))

	for _, item := range aclItems {
		// aclitem text form: "grantee=privileges/grantor", e.g. "phoenix_tenant=ar/postgres".
		grantee, rest, found := strings.Cut(item, "=")
		if !found || grantee != "phoenix_tenant" {
			continue
		}
		privileges, _, _ := strings.Cut(rest, "/")
		assert.NotContainsf(t, privileges, "w",
			"default ACL on schema audit still grants phoenix_tenant UPDATE (%s) — "+
				"every future audit table would start mutable", item)
		assert.NotContainsf(t, privileges, "d",
			"default ACL on schema audit still grants phoenix_tenant DELETE (%s) — "+
				"every future audit table would start deletable", item)
	}
}
