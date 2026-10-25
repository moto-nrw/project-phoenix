package migrations

import (
	"context"
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// protocolSeeds insert one row of each audit protocol of 1.15.454 for the tenant
// ?0, naming the account ?1 wherever the protocol keeps a person, dated by
// the SQL expression they are formatted with.
var protocolSeeds = map[string]string{
	"audit.student_deletions": `INSERT INTO audit.student_deletions (tenant_id, student_id, actor_account_id, reason, counts, deleted_at)
		VALUES (?0, ?1, ?1, 'test_data', '{}'::jsonb, %s) RETURNING id`,
	"audit.enrollment_deletions": `INSERT INTO audit.enrollment_deletions (tenant_id, request_id, actor_account_id, actor_type, scope, reason, counts, deleted_at)
		VALUES (?0, ?1, ?1, 'admin', 'request', 'Testanmeldung', '{}'::jsonb, %s) RETURNING id`,
	"audit.enrollment_restorations": `INSERT INTO audit.enrollment_restorations (tenant_id, request_id, child_ids, actor_account_id, restored_at)
		VALUES (?0, ?1, '[]'::jsonb, ?1, %s) RETURNING id`,
	"audit.time_tracking_deletions": `INSERT INTO audit.time_tracking_deletions (tenant_id, staff_id, source, source_id, deleted_by, payload, occurred_at)
		VALUES (?0, ?1, 'absence', ?1, ?1, '{}'::jsonb, %s) RETURNING id`,
	"audit.staff_master_data_changes": `INSERT INTO audit.staff_master_data_changes (tenant_id, staff_id, changed_by, section, field_name, occurred_at)
		VALUES (?0, ?1, ?1, 'contact', 'email', %s) RETURNING id`,
	"audit.personnel_number_changes": `INSERT INTO audit.personnel_number_changes (tenant_id, staff_id, changed_by, occurred_at)
		VALUES (?0, ?1, ?1, %s) RETURNING id`,
	"audit.guardian_financial_changes": `INSERT INTO audit.guardian_financial_changes (tenant_id, guardian_profile_id, changed_by, field_name, occurred_at)
		VALUES (?0, ?1, ?1, 'iban', %s) RETURNING id`,
	"audit.class_list_entry_changes": `INSERT INTO audit.class_list_entry_changes (tenant_id, entry_id, action, matched_student_id, changed_by, occurred_at)
		VALUES (?0, ?1, 'assigned', ?1, ?1, %s) RETURNING id`,
	"audit.substitution_changes": `INSERT INTO audit.substitution_changes (tenant_id, substitution_id, target_type, action, group_id, target_staff_id, actor_account_id, start_date, end_date, created_at)
		VALUES (?0, ?1, 'group_handover', 'assigned', ?1, ?1, ?1, CURRENT_DATE, CURRENT_DATE, %s) RETURNING id`,
	"audit.file_events": `INSERT INTO audit.file_events (tenant_id, action, actor_account_id, actor_name, detail, created_at)
		VALUES (?0, 'file_uploaded', ?1, 'Testkraft', 'liste.pdf', %s) RETURNING id`,
	"audit.export_transfers": `INSERT INTO audit.export_transfers (tenant_id, actor_account_id, actor_name, export_kind, export_format, filename,
			byte_size, target_host, target_port, target_directory, status, created_at)
		VALUES (?0, ?1, 'Testkraft', 'staff_time_tracking', 'csv', 'zeiten.csv', 1, 'sftp.example.org', 22, '/', 'success', %s) RETURNING id`,
}

// retentionProtocol is one protocol of 1.15.454: its deletion-log kind, its
// table, and whether it stays until the end of the following year instead
// of three years. The export journal has its own function.
type retentionProtocol struct {
	kind          string
	table         string
	followingYear bool
}

var retentionProtocols = []retentionProtocol{
	{"student_deletion_log_retention", "audit.student_deletions", false},
	{"enrollment_deletion_log_retention", "audit.enrollment_deletions", false},
	{"enrollment_restoration_log_retention", "audit.enrollment_restorations", false},
	{"time_tracking_deletion_log_retention", "audit.time_tracking_deletions", false},
	{"staff_master_data_change_retention", "audit.staff_master_data_changes", false},
	{"personnel_number_change_retention", "audit.personnel_number_changes", false},
	{"guardian_financial_change_retention", "audit.guardian_financial_changes", false},
	{"class_list_change_retention", "audit.class_list_entry_changes", true},
	{"substitution_change_retention", "audit.substitution_changes", true},
	{"file_event_retention", "audit.file_events", true},
	{"export_transfer_retention", "audit.export_transfers", true},
}

// call is the SQL that runs the protocol's function for the tenant ? up to
// cutoffSQL.
func (p retentionProtocol) call(cutoffSQL string) string {
	if p.kind == "export_transfer_retention" {
		return fmt.Sprintf("SELECT audit.delete_expired_export_transfers(?, %s)", cutoffSQL)
	}
	return fmt.Sprintf("SELECT audit.delete_expired_protocol(?, '%s', %s)", p.kind, cutoffSQL)
}

// dates are the window boundaries of the protocol as SQL: the newest cutoff
// the function allows, a row just past the window and a row just inside it.
func (p retentionProtocol) dates() (allowedCutoff, expired, fresh string) {
	if p.followingYear {
		return lastYearStartSQL, lastYearStartSQL + " - INTERVAL '1 day'", lastYearStartSQL + " + INTERVAL '1 day'"
	}
	return "(CURRENT_TIMESTAMP AT TIME ZONE 'UTC' - INTERVAL '3 years') AT TIME ZONE 'UTC'",
		"CURRENT_TIMESTAMP - INTERVAL '3 years 1 day'",
		"CURRENT_TIMESTAMP - INTERVAL '3 years' + INTERVAL '1 day'"
}

func runProtocolRetention(t *testing.T, db *testpkg.DB, transactionTenantID int64, query string, argumentTenantID int64) (int64, error) {
	t.Helper()
	var deleted int64
	err := withPhoenixTenantTx(t, db, transactionTenantID, func(ctx context.Context, tx testpkg.Tx) error {
		return tx.NewRaw(query, argumentTenantID).Scan(ctx, &deleted)
	})
	return deleted, err
}

func seedProtocolRow(t *testing.T, db *testpkg.DB, table, at string, tenantID, accountID int64) int64 {
	t.Helper()
	seed, ok := protocolSeeds[table]
	require.True(t, ok, "no seed for %s", table)
	var id int64
	require.NoError(t, db.NewRaw(fmt.Sprintf(seed, at), tenantID, accountID).Scan(context.Background(), &id))
	return id
}

func TestProtocolRetentionRejectsForeignSchoolsAndYoungCutoffs(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	authDB := testpkg.SetupServeTestDB(t)
	t.Cleanup(func() { require.NoError(t, authDB.Close()) })
	account := testpkg.CreateTestAccount(t, db, "protocol-retention@example.com").ID

	for _, protocol := range retentionProtocols {
		t.Run(protocol.kind, func(t *testing.T) {
			allowedCutoff, expired, _ := protocol.dates()
			tenantA, _ := testpkg.CreateTestTenant(t, db)
			tenantB, _ := testpkg.CreateTestTenant(t, db)
			rowA := seedProtocolRow(t, db, protocol.table, expired, tenantA, account)
			rowB := seedProtocolRow(t, db, protocol.table, expired, tenantB, account)

			_, err := runProtocolRetention(t, authDB, tenantA, protocol.call(allowedCutoff), tenantB)
			require.ErrorContains(t, err, "SQLSTATE=42501", "a school never deletes another school's rows")
			_, err = runProtocolRetention(t, authDB, tenantA, protocol.call("CURRENT_TIMESTAMP"), tenantA)
			require.ErrorContains(t, err, "SQLSTATE=42501", "nothing younger than the fixed window is deleted")
			assertAuditRowsExist(t, db, protocol.table, rowA, rowB)
		})
	}
}

func TestProtocolRetentionRejectsAnUnknownProtocol(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	authDB := testpkg.SetupServeTestDB(t)
	t.Cleanup(func() { require.NoError(t, authDB.Close()) })
	tenantID, _ := testpkg.CreateTestTenant(t, db)

	_, err := runProtocolRetention(t, authDB, tenantID,
		"SELECT audit.delete_expired_protocol(?, 'auth_event_retention', CURRENT_TIMESTAMP - INTERVAL '10 years')", tenantID)
	require.ErrorContains(t, err, "SQLSTATE=22023", "the shared function deletes only the protocols it names")
}

func TestProtocolRetentionDeletesOnlyExpiredRowsOfTheOwnSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	authDB := testpkg.SetupServeTestDB(t)
	t.Cleanup(func() { require.NoError(t, authDB.Close()) })
	account := testpkg.CreateTestAccount(t, db, "protocol-retention-own@example.com").ID

	for _, protocol := range retentionProtocols {
		t.Run(protocol.kind, func(t *testing.T) {
			allowedCutoff, expired, fresh := protocol.dates()
			tenantA, _ := testpkg.CreateTestTenant(t, db)
			tenantB, _ := testpkg.CreateTestTenant(t, db)
			expiredA := seedProtocolRow(t, db, protocol.table, expired, tenantA, account)
			freshA := seedProtocolRow(t, db, protocol.table, fresh, tenantA, account)
			expiredB := seedProtocolRow(t, db, protocol.table, expired, tenantB, account)

			deleted, err := runProtocolRetention(t, authDB, tenantA, protocol.call(allowedCutoff), tenantA)
			require.NoError(t, err)
			assert.EqualValues(t, 1, deleted)
			assertAuditRowsMissing(t, db, protocol.table, expiredA)
			assertAuditRowsExist(t, db, protocol.table, freshA, expiredB)
		})
	}
}

func TestProtocolsStayClosedToDirectDeletes(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	for _, protocol := range retentionProtocols {
		var canDelete bool
		require.NoError(t, db.NewRaw(`SELECT has_table_privilege('phoenix_tenant', ?, 'DELETE')`, protocol.table).Scan(context.Background(), &canDelete))
		assert.False(t, canDelete, "%s stays append-only for the tenant role", protocol.table)
	}
}

func TestSettingProtocolKeepsTheChangeOfADeletedAccount(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	account := testpkg.CreateTestAccount(t, db, "setting-protocol-actor@example.com").ID
	var entry int64
	require.NoError(t, db.NewRaw(`INSERT INTO config.setting_audit (tenant_id, setting_key, new_value, action, changed_by)
		VALUES (?, 'gdpr.data_cleanup_time', '"03:00"'::jsonb, 'set', ?) RETURNING id`, tenantID, account).Scan(context.Background(), &entry))

	_, err := db.ExecContext(context.Background(), `DELETE FROM auth.accounts WHERE id = ?`, account)
	require.NoError(t, err, "a setting change no longer blocks deleting its account")

	var changedBy *int64
	require.NoError(t, db.NewRaw(`SELECT changed_by FROM config.setting_audit WHERE id = ?`, entry).Scan(context.Background(), &changedBy))
	assert.Nil(t, changedBy, "the change stays and loses only who made it")
}
