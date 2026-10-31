package migrations

import (
	"context"
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func insertPayrollDispatch(t *testing.T, db *bun.DB, schoolID int64) {
	t.Helper()
	_, err := db.ExecContext(t.Context(), `INSERT INTO active.payroll_dispatches
		(tenant_id, year, month, sequence, channel, export_format, recipient_email, actor_account_id, state,
		 filename, file_data, download_token_hash, download_expires_at)
		VALUES (?, 2026, 9, 0, 'email', 'csv', 'lohn@steuerbuero-beispiel.de', 31, 'mailing',
		 'zeiterfassung_alle_2026-09.csv', 'Zeiten', ?, now() + interval '14 days')`,
		schoolID, fmt.Sprintf("fp-%d", schoolID))
	require.NoError(t, err)
}

func countPayrollDispatches(t *testing.T, db *bun.DB, schoolID int64) int {
	t.Helper()
	var rows int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM active.payroll_dispatches WHERE tenant_id = ?`, schoolID).Scan(t.Context(), &rows))
	return rows
}

// TestSchoolErasureRemovesPayrollDispatches pins the erasure path of
// 1.15.718 (#4805): the plan deletes a school's dispatches, payroll file
// included, and the other school keeps its own.
func TestSchoolErasureRemovesPayrollDispatches(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	school := testpkg.UniqueTestTenantID(t)
	other := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, school)
	testpkg.EnsureTestTenant(t, db, other)
	insertPayrollDispatch(t, db, school)
	insertPayrollDispatch(t, db, other)

	result := eraseSchoolAsCLI(t, db, school, beginErasure(t, db, school))

	require.Equal(t, "db_done", result.Status, "preflight, deletion and residual checks pass")
	assert.Zero(t, countPayrollDispatches(t, db, school))
	assert.Equal(t, 1, countPayrollDispatches(t, db, other))
}

// TestPayrollDispatchesMigrationRoundTrip pins that 1.15.718 runs twice,
// rolls back to the 1.15.705 erasure without the table and the journal
// columns, and applies again.
func TestPayrollDispatchesMigrationRoundTrip(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	exists := func() bool {
		var found bool
		require.NoError(t, db.NewRaw(`SELECT to_regclass('active.payroll_dispatches') IS NOT NULL`).Scan(ctx, &found))
		return found
	}
	channelColumn := func() bool {
		var found bool
		require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT FROM information_schema.columns
			WHERE table_schema = 'audit' AND table_name = 'export_transfers' AND column_name = 'export_channel')`).Scan(ctx, &found))
		return found
	}
	require.NoError(t, payrollDispatchesUp(ctx, db), "the migration is idempotent")
	// The Elternsprechtag (1.15.719, #4801) extends this erasure; it goes first.
	require.NoError(t, parentConferencesDown(ctx, db))
	require.NoError(t, payrollDispatchesDown(ctx, db))
	assert.False(t, exists())
	assert.False(t, channelColumn())
	for _, entry := range readErasurePlan(t, db) {
		assert.NotEqual(t, "active.payroll_dispatches", entry.Name)
	}
	school := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, school)
	result := eraseSchoolAsCLI(t, db, school, beginErasure(t, db, school))
	require.Equal(t, "db_done", result.Status, "the restored erasure still works")

	require.NoError(t, payrollDispatchesUp(ctx, db))
	assert.True(t, exists())
	assert.True(t, channelColumn())
	require.NoError(t, parentConferencesUp(ctx, db))
}

// TestExportTransferJournalNamesTheRecipientOfAnEmailSend pins the CHECK of
// the journal: an e-mail send names its recipient, an SFTP send needs none.
func TestExportTransferJournalNamesTheRecipientOfAnEmailSend(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	school := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, school)
	insert := func(channel, recipient string) error {
		_, err := db.ExecContext(t.Context(), `INSERT INTO audit.export_transfers
			(tenant_id, actor_name, export_kind, export_format, filename, byte_size, target_host, target_port,
			 target_directory, status, export_channel, export_period, target_email)
			VALUES (?, 'Lea', 'staff_time_tracking', 'csv', 'datei.csv', 6, '', 0, '', 'success', ?, '2026-09', ?)`,
			school, channel, recipient)
		return err
	}
	require.NoError(t, insert("email", "lohn@steuerbuero-beispiel.de"))
	require.Error(t, insert("email", ""), "an e-mail send without recipient says nothing about where the file went")
	require.Error(t, insert("fax", ""))
}
