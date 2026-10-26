package behavior_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestPermissionAdministrationRollback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	identity := roleAdministrationOf(t, setupAuthService(t, db))
	role := testpkg.CreateTestRole(t, db, "permission-owner")
	roleGrant := testpkg.CreateTestPermission(t, db, "permission-role", "role", "read")
	require.NoError(t, identity.AssignPermissionToRole(ctx, role.ID, roleGrant.ID))
	require.NoError(t, identity.AssignPermissionToRole(ctx, role.ID, roleGrant.ID), "assigning twice is idempotent")

	rollback := errors.New("roll back permission changes")
	err := testpkg.WithAdminTx(t, ctx, db, func(txCtx context.Context, _ bun.Tx) error {
		txCtx = testpkg.ContextForTenant(txCtx, tenantID)
		require.NoError(t, identity.RemovePermissionFromRole(txCtx, role.ID, roleGrant.ID))
		permissions, queryErr := identity.GetRolePermissions(txCtx, role.ID)
		require.NoError(t, queryErr)
		require.Empty(t, permissions)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	permissions, err := identity.GetRolePermissions(ctx, role.ID)
	require.NoError(t, err)
	require.Equal(t, []int64{roleGrant.ID}, permissionIDs(permissions))
}

func permissionIDs(permissions []identityaccess.Permission) []int64 {
	ids := make([]int64, 0, len(permissions))
	for _, permission := range permissions {
		ids = append(ids, permission.ID)
	}
	return ids
}

func TestPermissionAdministrationDatabaseFailure(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupClosableTestDB(t)
	identity := roleAdministrationOf(t, setupAuthService(t, db))
	require.NoError(t, db.Close())
	ctx := testpkg.Ctx(t)
	_, err := identity.GetPermission(ctx, 1)
	require.ErrorContains(t, err, "database is closed")
	_, err = identity.ListPermissions(ctx, identityaccess.PermissionFilter{})
	require.ErrorContains(t, err, "database is closed")
}
