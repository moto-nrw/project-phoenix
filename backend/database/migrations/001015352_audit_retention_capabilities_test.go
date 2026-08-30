package migrations

import (
	"context"
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type auditRetentionCapability struct {
	name              string
	function          string
	allowedCutoffSQL  string
	bypassCutoffSQL   string
	seedExpiredRowSQL string
	seedFreshRowSQL   string
	usesStudentID     bool
	table             string
}

var auditRetentionCapabilities = []auditRetentionCapability{
	{
		name:             "deviation events",
		function:         "audit.delete_expired_deviation_events",
		allowedCutoffSQL: "(CURRENT_TIMESTAMP AT TIME ZONE 'Europe/Berlin')::date - 30",
		bypassCutoffSQL:  "(CURRENT_TIMESTAMP AT TIME ZONE 'Europe/Berlin')::date",
		seedExpiredRowSQL: `INSERT INTO audit.deviation_events
			(tenant_id, occurrence_date, start_time, event_type)
			VALUES (?, (CURRENT_TIMESTAMP AT TIME ZONE 'Europe/Berlin')::date - 60, '12:00', 'retention_test')
			RETURNING id`,
		seedFreshRowSQL: `INSERT INTO audit.deviation_events
			(tenant_id, occurrence_date, start_time, event_type)
			VALUES (?, (CURRENT_TIMESTAMP AT TIME ZONE 'Europe/Berlin')::date - 10, '12:00', 'retention_test')
			RETURNING id`,
		table: "audit.deviation_events",
	},
	{
		name:     "student field edits",
		function: "audit.delete_expired_student_field_edits",
		allowedCutoffSQL: `(((CURRENT_TIMESTAMP AT TIME ZONE 'Europe/Berlin')::date - 30)::timestamp
			AT TIME ZONE 'Europe/Berlin')`,
		bypassCutoffSQL: `((CURRENT_TIMESTAMP AT TIME ZONE 'Europe/Berlin')::date::timestamp
			AT TIME ZONE 'Europe/Berlin')`,
		seedExpiredRowSQL: `INSERT INTO audit.student_field_edits
			(tenant_id, student_id, edited_by, edited_by_name, field_name, created_at)
			VALUES (?, ?, 0, 'System', 'status', CURRENT_TIMESTAMP - INTERVAL '60 days')
			RETURNING id`,
		seedFreshRowSQL: `INSERT INTO audit.student_field_edits
			(tenant_id, student_id, edited_by, edited_by_name, field_name, created_at)
			VALUES (?, ?, 0, 'System', 'status', CURRENT_TIMESTAMP - INTERVAL '10 days')
			RETURNING id`,
		usesStudentID: true,
		table:         "audit.student_field_edits",
	},
	{
		name:             "unregistered tag scans",
		function:         "audit.delete_expired_unregistered_tag_scans",
		allowedCutoffSQL: "CURRENT_TIMESTAMP - INTERVAL '90 days'",
		bypassCutoffSQL:  "CURRENT_TIMESTAMP",
		seedExpiredRowSQL: `INSERT INTO audit.unregistered_tag_scans
			(tenant_id, tag_uid, scanned_at)
			VALUES (?, 'retention-expired', CURRENT_TIMESTAMP - INTERVAL '120 days')
			RETURNING id`,
		seedFreshRowSQL: `INSERT INTO audit.unregistered_tag_scans
			(tenant_id, tag_uid, scanned_at)
			VALUES (?, 'retention-fresh', CURRENT_TIMESTAMP - INTERVAL '10 days')
			RETURNING id`,
		table: "audit.unregistered_tag_scans",
	},
}

func TestAuditRetentionCapabilitiesRejectCrossTenantAndPredicateBypass(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	authDB := testpkg.SetupServeTestDB(t)
	t.Cleanup(func() { require.NoError(t, authDB.Close()) })

	for _, capability := range auditRetentionCapabilities {
		t.Run(capability.name, func(t *testing.T) {
			tenantA, studentA := retentionCapabilityTenant(t, db)
			tenantB, studentB := retentionCapabilityTenant(t, db)
			setAuditRetentionOverrides(t, db, tenantA)
			setAuditRetentionOverrides(t, db, tenantB)

			rowA := seedAuditRetentionRow(t, db, capability.seedExpiredRowSQL, capability.usesStudentID, tenantA, studentA)
			rowB := seedAuditRetentionRow(t, db, capability.seedExpiredRowSQL, capability.usesStudentID, tenantB, studentB)

			crossTenantErr := callAuditRetentionCapability(
				t, authDB, tenantA, capability.function, tenantB, capability.allowedCutoffSQL,
			)
			require.Error(t, crossTenantErr, "capability must reject a tenant ID other than the transaction tenant")
			require.ErrorContains(t, crossTenantErr, "SQLSTATE=42501")
			assertAuditRowsExist(t, db, capability.table, rowA, rowB)

			bypassErr := callAuditRetentionCapability(
				t, authDB, tenantA, capability.function, tenantA, capability.bypassCutoffSQL,
			)
			require.Error(t, bypassErr, "capability must reject a cutoff newer than the effective retention window")
			require.ErrorContains(t, bypassErr, "SQLSTATE=42501")
			assertAuditRowsExist(t, db, capability.table, rowA, rowB)
		})
	}
}

func TestAuditRetentionCapabilitiesDeleteOnlyExpiredRowsForTransactionTenant(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	authDB := testpkg.SetupServeTestDB(t)
	t.Cleanup(func() { require.NoError(t, authDB.Close()) })

	for _, capability := range auditRetentionCapabilities {
		t.Run(capability.name, func(t *testing.T) {
			tenantA, studentA := retentionCapabilityTenant(t, db)
			tenantB, studentB := retentionCapabilityTenant(t, db)
			setAuditRetentionOverrides(t, db, tenantA)
			setAuditRetentionOverrides(t, db, tenantB)

			expiredA := seedAuditRetentionRow(t, db, capability.seedExpiredRowSQL, capability.usesStudentID, tenantA, studentA)
			freshA := seedAuditRetentionRow(t, db, capability.seedFreshRowSQL, capability.usesStudentID, tenantA, studentA)
			expiredB := seedAuditRetentionRow(t, db, capability.seedExpiredRowSQL, capability.usesStudentID, tenantB, studentB)

			var deleted int64
			err := withPhoenixTenantTx(t, authDB, tenantA, func(ctx context.Context, tx testpkg.Tx) error {
				return tx.NewRaw(
					fmt.Sprintf("SELECT %s(?, %s)", capability.function, capability.allowedCutoffSQL),
					tenantA,
				).Scan(ctx, &deleted)
			})
			require.NoError(t, err)
			assert.EqualValues(t, 1, deleted)
			assertAuditRowsMissing(t, db, capability.table, expiredA)
			assertAuditRowsExist(t, db, capability.table, freshA, expiredB)
		})
	}
}

func TestAuditRetentionCapabilitiesRejectMissingRetentionSettings(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	authDB := testpkg.SetupServeTestDB(t)
	t.Cleanup(func() { require.NoError(t, authDB.Close()) })

	for _, capability := range auditRetentionCapabilities[:2] {
		t.Run(capability.name, func(t *testing.T) {
			tenantID, studentID := retentionCapabilityTenant(t, db)
			rowID := seedAuditRetentionRow(t, db, capability.seedExpiredRowSQL, capability.usesStudentID, tenantID, studentID)

			err := callAuditRetentionCapability(
				t, authDB, tenantID, capability.function, tenantID, capability.allowedCutoffSQL,
			)
			require.Error(t, err, "capability must fail closed when its retention setting is unresolved")
			require.ErrorContains(t, err, "SQLSTATE=22023")
			assertAuditRowsExist(t, db, capability.table, rowID)
		})
	}
}

func retentionCapabilityTenant(t *testing.T, db *testpkg.DB) (int64, int64) {
	t.Helper()
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Retention", "Boundary", "1a")
	return tenantID, student.ID
}

func setAuditRetentionOverrides(t *testing.T, db *testpkg.DB, tenantID int64) {
	t.Helper()
	_, err := db.NewRaw(`
		INSERT INTO config.setting_values (tenant_id, setting_key, value)
		VALUES (?, 'gdpr.timetable_retention_days', '30'::jsonb),
		       (?, 'gdpr.student_change_log_retention_days', '30'::jsonb)
		ON CONFLICT (tenant_id, setting_key) DO UPDATE SET value = EXCLUDED.value
	`, tenantID, tenantID).Exec(context.Background())
	require.NoError(t, err)
}

func seedAuditRetentionRow(
	t *testing.T, db *testpkg.DB, query string, usesStudentID bool, tenantID, studentID int64,
) int64 {
	t.Helper()
	args := []any{tenantID}
	if usesStudentID {
		args = append(args, studentID)
	}
	var id int64
	require.NoError(t, db.NewRaw(query, args...).Scan(context.Background(), &id))
	return id
}

func callAuditRetentionCapability(
	t *testing.T,
	db *testpkg.DB,
	transactionTenantID int64,
	function string,
	argumentTenantID int64,
	cutoffSQL string,
) error {
	t.Helper()
	return withPhoenixTenantTx(t, db, transactionTenantID, func(ctx context.Context, tx testpkg.Tx) error {
		var deleted int64
		return tx.NewRaw(fmt.Sprintf("SELECT %s(?, %s)", function, cutoffSQL), argumentTenantID).
			Scan(ctx, &deleted)
	})
}

func withPhoenixTenantTx(
	t *testing.T,
	db *testpkg.DB,
	tenantID int64,
	fn func(context.Context, testpkg.Tx) error,
) error {
	t.Helper()
	ctx := t.Context()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err = tx.NewRaw(`SELECT set_config('app.current_tenant_id', ?, true)`, fmt.Sprint(tenantID)).Exec(ctx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err = fn(ctx, tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func assertAuditRowsExist(t *testing.T, db *testpkg.DB, table string, ids ...int64) {
	t.Helper()
	count, err := db.NewSelect().TableExpr(table).Where("id IN (?)", testpkg.DBList(ids)).Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, len(ids), count)
}

func assertAuditRowsMissing(t *testing.T, db *testpkg.DB, table string, ids ...int64) {
	t.Helper()
	count, err := db.NewSelect().TableExpr(table).Where("id IN (?)", testpkg.DBList(ids)).Count(context.Background())
	require.NoError(t, err)
	assert.Zero(t, count)
}
