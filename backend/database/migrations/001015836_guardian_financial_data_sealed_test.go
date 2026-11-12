package migrations

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// #6332: once a sealed value exists, the sealed columns hold the only copy,
// so neither rollback may drop them; without one the rollback proceeds.
func TestSealedBankDataRollbackKeepsTheOnlyCopy(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	school := testpkg.NewTenantScope(t, db).TenantID
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, school, "Rollback", "Guardian", "rollback-guardian@example.invalid")
	member := testpkg.CreateTestStaffForTenant(t, db, school, "Rollback", "Staff")
	_, err := db.NewRaw(`INSERT INTO users.guardian_financial_data (tenant_id, guardian_profile_id, account_holder_sealed) VALUES (?, ?, '\x01')`, school, guardian.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw(`INSERT INTO users.staff_financial_data (tenant_id, staff_id, social_security_number_sealed) VALUES (?, ?, '\x01')`, school, member.ID).Exec(ctx)
	require.NoError(t, err)

	require.ErrorIs(t, staffFinancialDataSealedDown(ctx, db), errSealedBankDataRollback)
	require.ErrorIs(t, guardianFinancialDataSealedDown(ctx, db), errSealedBankDataRollback)
	var kept int
	require.NoError(t, db.NewRaw(`SELECT (SELECT count(*) FROM users.guardian_financial_data WHERE account_holder_sealed IS NOT NULL)
   + (SELECT count(*) FROM users.staff_financial_data WHERE social_security_number_sealed IS NOT NULL)`).Scan(ctx, &kept))
	require.Equal(t, 2, kept)

	_, err = db.NewRaw(`DELETE FROM users.staff_financial_data WHERE staff_id = ?; DELETE FROM users.guardian_financial_data WHERE guardian_profile_id = ?`, member.ID, guardian.ID).Exec(ctx)
	require.NoError(t, err)
	require.NoError(t, staffFinancialDataSealedDown(ctx, db))
	require.NoError(t, guardianFinancialDataSealedDown(ctx, db))
	require.NoError(t, guardianFinancialDataSealedUp(ctx, db))
	require.NoError(t, staffFinancialDataSealedUp(ctx, db))
}

// #6332 invariant: the rollback locks the table before it checks, so a
// writer still open at the check cannot have its sealed value dropped after
// its commit. Red when the check runs before the lock.
func TestSealedGuardianBankDataRollbackWaitsForAnOpenWriter(t *testing.T) {
	t.Parallel()
	{
		db := testpkg.SetupIsolatedTestDB(t)
		school := testpkg.NewTenantScope(t, db).TenantID
		guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, school, "Racing", "Guardian", "racing-guardian@example.invalid")
		err := runAgainstOpenWriter(t, db,
			`INSERT INTO users.guardian_financial_data (tenant_id, guardian_profile_id, iban_sealed) VALUES (?, ?, '\x01')`,
			[]any{school, guardian.ID}, func(ctx context.Context) error { return guardianFinancialDataSealedDown(ctx, db) })
		require.ErrorIs(t, err, errSealedBankDataRollback)
		require.True(t, bankColumnExists(t, db, "guardian_financial_data", "iban_sealed"), "the only copy stays")
	}
}

// The same invariant for the staff rollback.
func TestSealedStaffBankDataRollbackWaitsForAnOpenWriter(t *testing.T) {
	t.Parallel()
	{
		db := testpkg.SetupIsolatedTestDB(t)
		school := testpkg.NewTenantScope(t, db).TenantID
		member := testpkg.CreateTestStaffForTenant(t, db, school, "Racing", "Staff")
		err := runAgainstOpenWriter(t, db,
			`INSERT INTO users.staff_financial_data (tenant_id, staff_id, tax_id_sealed) VALUES (?, ?, '\x01')`,
			[]any{school, member.ID}, func(ctx context.Context) error { return staffFinancialDataSealedDown(ctx, db) })
		require.ErrorIs(t, err, errSealedBankDataRollback)
		require.True(t, bankColumnExists(t, db, "staff_financial_data", "tax_id_sealed"), "the only copy stays")
	}
}

// runAgainstOpenWriter holds a writer transaction open while drop runs, waits
// until drop blocks on a lock, then commits the writer and returns drop's
// result. A migration that checks before it locks sees nothing at its check,
// waits only at its DROP and then removes the writer's committed value.
func runAgainstOpenWriter(t *testing.T, db *bun.DB, writerSQL string, writerArgs []any, drop func(context.Context) error) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	writer, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	committed := false
	defer func() {
		if !committed {
			_ = writer.Rollback()
		}
	}()
	_, err = writer.ExecContext(ctx, writerSQL, writerArgs...)
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() { done <- drop(ctx) }()
	require.Eventually(t, func() bool {
		var waiting bool
		err := db.NewRaw(`SELECT EXISTS (SELECT 1 FROM pg_stat_activity
   WHERE datname = current_database() AND wait_event_type = 'Lock' AND pid <> pg_backend_pid())`).Scan(ctx, &waiting)
		return err == nil && waiting
	}, 10*time.Second, 10*time.Millisecond, "the migration must wait for the open writer")
	select {
	case err := <-done:
		t.Fatalf("the migration finished while the writer was open: %v", err)
	default:
	}
	require.NoError(t, writer.Commit())
	committed = true
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		t.Fatal("the migration did not finish after the writer committed")
		return nil
	}
}

func bankColumnExists(t *testing.T, db *bun.DB, table, column string) bool {
	t.Helper()
	var exists bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM information_schema.columns
   WHERE table_schema = 'users' AND table_name = ? AND column_name = ?)`, table, column).Scan(context.Background(), &exists))
	return exists
}
