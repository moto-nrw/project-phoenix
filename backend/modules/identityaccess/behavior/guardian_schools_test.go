package behavior_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestGuardianSchoolsEligibilityAndTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	identity, err := repositories.NewIdentityAccessForTests(db)
	require.NoError(t, err)
	account := testpkg.CreateTestAccount(t, db, "guardian-schools")
	ctx := testpkg.Ctx(t)
	var guardianRoleID int64
	require.NoError(t, db.NewRaw(`SELECT id FROM auth.roles WHERE name = 'guardian' AND is_system = TRUE`).Scan(ctx, &guardianRoleID))
	ids := make([]int64, 4)
	for i := range ids {
		ids[i] = testpkg.UniqueTestTenantID(t)
		testpkg.EnsureTestTenant(t, db, ids[i])
		testpkg.MapAccountToTenant(t, db, account.ID, ids[i])
		// Explicit ordering, independent of wall-clock and fixture execution.
		_, err = db.NewRaw(`UPDATE auth.account_tenants SET created_at = TIMESTAMPTZ '2025-01-01' + (? * INTERVAL '1 day') WHERE account_id = ? AND tenant_id = ?`, i, account.ID, ids[i]).Exec(ctx)
		require.NoError(t, err)
		if i != 2 {
			_, err = db.NewRaw(`INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)`, account.ID, guardianRoleID, ids[i]).Exec(ctx)
			require.NoError(t, err)
		}
	}
	_, err = db.NewRaw(`UPDATE auth.account_tenants SET status = 'inactive' WHERE account_id = ? AND tenant_id = ?`, account.ID, ids[1]).Exec(ctx)
	require.NoError(t, err)

	read := func(ctx context.Context) []int64 {
		t.Helper()
		var result []int64
		require.NoError(t, tenant.WithAdminTx(ctx, db, func(txCtx context.Context, _ bun.Tx) error {
			var readErr error
			result, readErr = identity.ListGuardianSchoolIDs(txCtx, account.ID)
			return readErr
		}))
		return result
	}
	// A guardian role at another school cannot authorize the role-less school;
	// an inactive membership is excluded even with the guardian role.
	require.Equal(t, []int64{ids[0], ids[3]}, read(ctx))
	rollback := errors.New("roll back guardian offboarding")
	err = tenant.WithAdminTx(ctx, db, func(txCtx context.Context, tx bun.Tx) error {
		_, writeErr := tx.NewRaw(`DELETE FROM auth.account_roles WHERE account_id = ? AND tenant_id = ?`, account.ID, ids[0]).Exec(txCtx)
		require.NoError(t, writeErr)
		actual, readErr := identity.ListGuardianSchoolIDs(txCtx, account.ID)
		require.NoError(t, readErr)
		require.Equal(t, []int64{ids[3]}, actual)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	require.Equal(t, []int64{ids[0], ids[3]}, read(ctx))
	missing, err := identity.ListGuardianSchoolIDs(ctx, 0)
	require.NoError(t, err)
	require.Empty(t, missing)
}

func TestGuardianSchoolsDatabaseFailure(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupClosableTestDB(t)
	identity, err := repositories.NewIdentityAccessForTests(db)
	require.NoError(t, err)
	require.NoError(t, db.Close())
	_, err = identity.ListGuardianSchoolIDs(context.Background(), 1)
	require.ErrorContains(t, err, "database is closed")
}
