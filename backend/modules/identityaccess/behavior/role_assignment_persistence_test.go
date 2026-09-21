package behavior_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Assignment persistence is exercised through the serving capability, not the
// retired generic mapping repository.
func TestNativeRoleAssignmentPersistenceLifecycle(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	roles := roleAdministrationOf(t, setupAuthService(t, db))
	ctx := testpkg.Ctx(t)
	home := testpkg.Tenant(t)
	other, _ := testpkg.CreateTestTenant(t, db)
	otherCtx := tenant.WithTenantID(ctx, other)
	account := testpkg.CreateTestAccount(t, db, "assignment-lifecycle")
	peer := testpkg.CreateTestAccount(t, db, "assignment-peer")
	testpkg.EnsureAccountTenant(t, db, account.ID, other)
	baseRole := "user"
	first, err := roles.CreateRole(ctx, "lifecycle-first", "first", &baseRole)
	require.NoError(t, err)
	second, err := roles.CreateRole(ctx, "lifecycle-second", "second", &baseRole)
	require.NoError(t, err)
	foreign, err := roles.CreateRole(otherCtx, "lifecycle-foreign", "foreign", &baseRole)
	require.NoError(t, err)
	initial, err := roles.GetAccountRoles(ctx, account.ID)
	require.NoError(t, err)
	require.Empty(t, initial)

	require.NoError(t, roles.AssignRoleToAccount(ctx, account.ID, first.ID))
	require.NoError(t, roles.AssignRoleToAccount(ctx, account.ID, first.ID))
	require.NoError(t, roles.AssignRoleToAccount(ctx, account.ID, second.ID))
	require.NoError(t, roles.AssignRoleToAccount(ctx, peer.ID, first.ID))
	require.NoError(t, roles.AssignRoleToAccount(otherCtx, account.ID, foreign.ID))
	current, err := roles.GetAccountRoles(ctx, account.ID)
	require.NoError(t, err)
	require.Len(t, current, 2, "repeated assignment must not duplicate the mapping")

	rollback := errors.New("roll back role replacement")
	err = tenant.WithAdminTx(ctx, db, func(txCtx context.Context, _ bun.Tx) error {
		txCtx = tenant.WithTenantID(txCtx, home)
		require.NoError(t, roles.ReplaceAccountRole(txCtx, account.ID, second.ID))
		inside, readErr := roles.GetAccountRoles(txCtx, account.ID)
		require.NoError(t, readErr)
		require.Len(t, inside, 1)
		require.Equal(t, second.ID, inside[0].ID)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	current, err = roles.GetAccountRoles(ctx, account.ID)
	require.NoError(t, err)
	require.Len(t, current, 2, "replacement must share the ambient transaction")

	require.NoError(t, roles.RemoveRoleFromAccount(ctx, account.ID, first.ID))
	require.NoError(t, roles.RemoveRoleFromAccount(ctx, account.ID, first.ID))
	current, err = roles.GetAccountRoles(ctx, account.ID)
	require.NoError(t, err)
	require.Len(t, current, 1)
	require.Equal(t, second.ID, current[0].ID)
	peerRoles, err := roles.GetAccountRoles(ctx, peer.ID)
	require.NoError(t, err)
	require.Len(t, peerRoles, 1, "removing one account's role must not affect another account")
	require.NoError(t, roles.ReplaceAccountRole(ctx, account.ID, first.ID))
	current, err = roles.GetAccountRoles(ctx, account.ID)
	require.NoError(t, err)
	require.Len(t, current, 1)
	require.Equal(t, first.ID, current[0].ID)
	require.NoError(t, roles.DeleteRole(ctx, first.ID))
	for _, accountID := range []int64{account.ID, peer.ID} {
		current, err = roles.GetAccountRoles(ctx, accountID)
		require.NoError(t, err)
		require.Empty(t, current, "deleting a role removes every local assignment")
	}
	_, err = roles.GetRole(ctx, first.ID)
	require.ErrorIs(t, err, identityaccess.ErrRoleNotFound)
	current, err = roles.GetAccountRoles(otherCtx, account.ID)
	require.NoError(t, err)
	require.Len(t, current, 1)
	require.Equal(t, foreign.ID, current[0].ID, "local mutations must preserve foreign school assignments")
}
