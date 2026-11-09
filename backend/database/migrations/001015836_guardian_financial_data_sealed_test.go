package migrations

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
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
