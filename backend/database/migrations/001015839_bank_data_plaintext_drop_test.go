package migrations

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestBankDataPlaintextDropPreflightSkipsPendingTableCreation(t *testing.T) {
	t.Parallel()
	ctx := withPendingMigrations(t.Context(), migrate.MigrationSlice{
		{Name: "001015336"}, // guardianPaymentDataVersion
		{Name: "001015251"}, // staffStammdatenVersion
	})
	require.NoError(t, bankDataPlaintextDropPrecondition(ctx, nil), "a fresh database has neither financial table yet")
}

func TestBankDataPlaintextDropPreflightChecksExistingStaffTable(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := withPendingMigrations(t.Context(), migrate.MigrationSlice{
		{Name: "001015336"}, // guardianPaymentDataVersion
	})
	require.NoError(t, bankDataPlaintextDropDown(ctx, db))
	_, err := db.ExecContext(ctx, `ALTER TABLE users.guardian_financial_data RENAME TO pending_guardian_financial_data`)
	require.NoError(t, err)
	school := testpkg.NewTenantScope(t, db).TenantID
	staff := testpkg.CreateTestStaffForTenant(t, db, school, "Pending", "Staff")
	_, err = db.ExecContext(ctx, `INSERT INTO users.staff_financial_data (tenant_id, staff_id, tax_id) VALUES (?, ?, 'synthetic-tax-id')`, school, staff.ID)
	require.NoError(t, err)

	require.ErrorIs(t, bankDataPlaintextDropPrecondition(ctx, db), errBankDataPlaintextRemains,
		"pending guardian table creation must not hide existing staff plaintext")
	_, err = db.ExecContext(ctx, `UPDATE users.staff_financial_data SET tax_id = NULL WHERE staff_id = ?`, staff.ID)
	require.NoError(t, err)
	require.NoError(t, bankDataPlaintextDropPrecondition(ctx, db))
	require.Error(t, bankDataPlaintextDropUp(ctx, db), "Up still requires both tables even with pending migrations in context")
}

func bankPlaintextColumns(t *testing.T, db *bun.DB) []string {
	t.Helper()
	var columns []string
	require.NoError(t, db.NewRaw(`SELECT table_name || '.' || column_name FROM information_schema.columns
   WHERE table_schema = 'users' AND table_name IN ('guardian_financial_data', 'staff_financial_data')
     AND column_name IN ('iban', 'account_holder', 'tax_id', 'social_security_number')
   ORDER BY 1`).Scan(context.Background(), &columns))
	return columns
}

// #6332, ADR 0188: the plaintext columns go only once bank-data-rekey has
// sealed every value. A remaining plaintext value of either table stops the
// preflight and the migration; without one the columns are dropped.
func TestBankDataPlaintextDropWaitsForTheBackfill(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	require.Empty(t, bankPlaintextColumns(t, db), "the migrated schema has no plaintext column")
	require.NoError(t, bankDataPlaintextDropDown(ctx, db))
	require.Len(t, bankPlaintextColumns(t, db), 5)

	school := testpkg.NewTenantScope(t, db).TenantID
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, school, "Drop", "Guardian", "drop-guardian@example.invalid")
	member := testpkg.CreateTestStaffForTenant(t, db, school, "Drop", "Staff")
	_, err := db.NewRaw(`INSERT INTO users.guardian_financial_data (tenant_id, guardian_profile_id, iban_sealed, iban_masked) VALUES (?, ?, '\x01', '•••• 3000')`, school, guardian.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw(`INSERT INTO users.staff_financial_data (tenant_id, staff_id, tax_id_sealed) VALUES (?, ?, '\x01')`, school, member.ID).Exec(ctx)
	require.NoError(t, err)

	for _, leftover := range []struct {
		sql   string
		owner int64
	}{
		{`UPDATE users.guardian_financial_data SET account_holder = 'Synthetic' WHERE guardian_profile_id = ?`, guardian.ID},
		{`UPDATE users.staff_financial_data SET social_security_number = '65170839J003' WHERE staff_id = ?`, member.ID},
	} {
		_, err = db.NewRaw(leftover.sql, leftover.owner).Exec(ctx)
		require.NoError(t, err)
		err = bankDataPlaintextDropPrecondition(ctx, db)
		require.ErrorIs(t, err, errBankDataPlaintextRemains)
		require.NotContains(t, err.Error(), "Synthetic")
		require.NotContains(t, err.Error(), "65170839J003")
		require.ErrorIs(t, bankDataPlaintextDropUp(ctx, db), errBankDataPlaintextRemains)
		require.Len(t, bankPlaintextColumns(t, db), 5, "a refused drop keeps every column")
		_, err = db.NewRaw(`UPDATE users.guardian_financial_data SET account_holder = NULL WHERE guardian_profile_id = ?;
UPDATE users.staff_financial_data SET social_security_number = NULL WHERE staff_id = ?`, guardian.ID, member.ID).Exec(ctx)
		require.NoError(t, err)
	}

	require.NoError(t, bankDataPlaintextDropPrecondition(ctx, db))
	require.NoError(t, bankDataPlaintextDropUp(ctx, db))
	require.Empty(t, bankPlaintextColumns(t, db))
	var kept int
	require.NoError(t, db.NewRaw(`SELECT (SELECT count(*) FROM users.guardian_financial_data WHERE iban_sealed IS NOT NULL)
   + (SELECT count(*) FROM users.staff_financial_data WHERE tax_id_sealed IS NOT NULL)`).Scan(ctx, &kept))
	require.Equal(t, 2, kept, "the sealed values stay")
}

// #6332 invariant: the drop locks both tables before it rechecks, so a
// writer of an old release still open at the check cannot lose its plaintext
// to the drop after its commit. Red when the recheck runs before the lock.
func TestBankDataPlaintextDropWaitsForAnOpenGuardianWriter(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	require.NoError(t, bankDataPlaintextDropDown(context.Background(), db))
	school := testpkg.NewTenantScope(t, db).TenantID
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, school, "Racing", "Drop", "racing-drop@example.invalid")
	err := runAgainstOpenWriter(t, db,
		`INSERT INTO users.guardian_financial_data (tenant_id, guardian_profile_id, account_holder) VALUES (?, ?, 'Synthetic holder')`,
		[]any{school, guardian.ID}, func(ctx context.Context) error { return bankDataPlaintextDropUp(ctx, db) })
	require.ErrorIs(t, err, errBankDataPlaintextRemains)
	require.True(t, bankColumnExists(t, db, "guardian_financial_data", "account_holder"), "the plaintext waits for the backfill")
}

// The same invariant for a staff writer.
func TestBankDataPlaintextDropWaitsForAnOpenStaffWriter(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	require.NoError(t, bankDataPlaintextDropDown(context.Background(), db))
	school := testpkg.NewTenantScope(t, db).TenantID
	member := testpkg.CreateTestStaffForTenant(t, db, school, "Racing", "Drop")
	err := runAgainstOpenWriter(t, db,
		`INSERT INTO users.staff_financial_data (tenant_id, staff_id, social_security_number) VALUES (?, ?, '65170839J003')`,
		[]any{school, member.ID}, func(ctx context.Context) error { return bankDataPlaintextDropUp(ctx, db) })
	require.ErrorIs(t, err, errBankDataPlaintextRemains)
	require.True(t, bankColumnExists(t, db, "staff_financial_data", "social_security_number"), "the plaintext waits for the backfill")
}
