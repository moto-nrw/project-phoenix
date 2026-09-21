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

func TestGuardianPortalMemberships(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	access, err := repositories.NewIdentityAccessForTests(db)
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	home := testpkg.Tenant(t)
	other, _ := testpkg.CreateTestTenant(t, db)
	active := testpkg.CreateTestParentGuardianChain(t, db)
	disabled := testpkg.CreateTestParentGuardianChain(t, db)
	departed := testpkg.CreateTestParentGuardianChain(t, db)
	withoutRole := testpkg.CreateTestParentGuardianChain(t, db)
	unrequested := testpkg.CreateTestParentGuardianChain(t, db)
	testpkg.EnsureAccountTenant(t, db, active.AccountID, other)
	_, err = db.ExecContext(ctx, `INSERT INTO auth.account_roles (account_id, role_id, tenant_id)
		SELECT ?, id, ? FROM auth.roles WHERE name = 'guardian' AND tenant_id IS NULL`, active.AccountID, other)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "UPDATE auth.accounts SET active = FALSE WHERE id = ?", disabled.AccountID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "UPDATE auth.account_tenants SET status = 'inactive' WHERE account_id = ? AND tenant_id = ?", departed.AccountID, home)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "DELETE FROM auth.account_roles WHERE account_id = ? AND tenant_id = ?", withoutRole.AccountID, home)
	require.NoError(t, err)
	ids := []int64{active.AccountID, disabled.AccountID, departed.AccountID, withoutRole.AccountID}
	memberships, err := access.FindActiveGuardianMemberships(ctx, ids)
	require.NoError(t, err)
	require.Len(t, memberships, 1)
	require.ElementsMatch(t, []int64{home, other}, memberships[active.AccountID])
	require.NotContains(t, memberships, unrequested.AccountID)

	require.NoError(t, testpkg.WithinTenantContext(t, ctx, db, home, func(txCtx context.Context) error {
		memberships, queryErr := access.FindActiveGuardianMemberships(txCtx, ids)
		require.NoError(t, queryErr)
		require.Equal(t, map[int64][]int64{active.AccountID: {home}}, memberships)
		return nil
	}))
	require.NoError(t, testpkg.WithinTenantContext(t, tenant.WithTenantID(ctx, other), db, other, func(txCtx context.Context) error {
		memberships, queryErr := access.FindActiveGuardianMemberships(txCtx, ids)
		require.NoError(t, queryErr)
		require.Equal(t, map[int64][]int64{active.AccountID: {other}}, memberships)
		return nil
	}))

	rollback := errors.New("roll back guardian membership revocation")
	err = tenant.WithAdminTx(ctx, db, func(txCtx context.Context, tx bun.Tx) error {
		_, deleteErr := tx.ExecContext(txCtx, "DELETE FROM auth.account_roles WHERE account_id = ? AND tenant_id = ?", active.AccountID, home)
		require.NoError(t, deleteErr)
		memberships, queryErr := access.FindActiveGuardianMemberships(txCtx, []int64{active.AccountID})
		require.NoError(t, queryErr)
		require.Equal(t, []int64{other}, memberships[active.AccountID], "another school's role cannot grant home-school access")
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	memberships, err = access.FindActiveGuardianMemberships(ctx, []int64{active.AccountID})
	require.NoError(t, err)
	require.ElementsMatch(t, []int64{home, other}, memberships[active.AccountID])
}

func TestGuardianPortalMembershipsEmptyAndDatabaseFailure(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupClosableTestDB(t)
	access, err := repositories.NewIdentityAccessForTests(db)
	require.NoError(t, err)
	require.NoError(t, db.Close())
	memberships, err := access.FindActiveGuardianMemberships(testpkg.Ctx(t), nil)
	require.NoError(t, err)
	require.NotNil(t, memberships)
	require.Empty(t, memberships)
	_, err = access.FindActiveGuardianMemberships(testpkg.Ctx(t), []int64{0})
	require.ErrorContains(t, err, "database is closed")
}
