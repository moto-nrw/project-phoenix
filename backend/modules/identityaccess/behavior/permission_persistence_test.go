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

func TestPermissionAdministrationScopeAndRollback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	other := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, other)
	identity := roleAdministrationOf(t, setupAuthService(t, db))
	otherCtx := tenant.WithTenantID(ctx, other)
	account := testpkg.CreateTestAccount(t, db, "permission-owner")
	testpkg.MapAccountToTenant(t, db, account.ID, other)
	role := testpkg.CreateTestRole(t, db, "permission-owner")
	direct := testpkg.CreateTestPermission(t, db, "permission-direct", "direct", "read")
	roleGrant := testpkg.CreateTestPermission(t, db, "permission-role", "role", "read")
	foreignGrant := testpkg.CreateTestPermission(t, db, "permission-foreign", "foreign", "read")
	_, err := db.NewRaw("INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)", account.ID, role.ID, tenantID).Exec(ctx)
	require.NoError(t, err)
	require.NoError(t, identity.AssignPermissionToRole(ctx, role.ID, roleGrant.ID))
	require.NoError(t, identity.AssignPermissionToRole(ctx, role.ID, roleGrant.ID), "assigning twice is idempotent")
	require.NoError(t, identity.GrantPermissionToAccount(ctx, account.ID, direct.ID))
	require.NoError(t, identity.GrantPermissionToAccount(otherCtx, account.ID, foreignGrant.ID))
	require.NoError(t, identity.GrantPermissionToAccount(otherCtx, account.ID, direct.ID))

	permissions, err := identity.GetAccountPermissions(ctx, account.ID)
	require.NoError(t, err)
	require.ElementsMatch(t, []int64{direct.ID, roleGrant.ID}, permissionIDs(permissions))
	permissions, err = identity.GetAccountDirectPermissions(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, []int64{direct.ID}, permissionIDs(permissions))

	require.NoError(t, identity.DenyPermissionToAccount(ctx, account.ID, direct.ID))
	permissions, err = identity.GetAccountDirectPermissions(ctx, account.ID)
	require.NoError(t, err)
	require.Empty(t, permissions)
	permissions, err = identity.GetAccountDirectPermissions(otherCtx, account.ID)
	require.NoError(t, err)
	require.ElementsMatch(t, []int64{direct.ID, foreignGrant.ID}, permissionIDs(permissions))

	rollback := errors.New("roll back permission changes")
	err = tenant.WithAdminTx(ctx, db, func(txCtx context.Context, _ bun.Tx) error {
		txCtx = tenant.WithTenantID(txCtx, tenantID)
		require.NoError(t, identity.GrantPermissionToAccount(txCtx, account.ID, direct.ID))
		require.NoError(t, identity.RemovePermissionFromRole(txCtx, role.ID, roleGrant.ID))
		permissions, queryErr := identity.GetAccountPermissions(txCtx, account.ID)
		require.NoError(t, queryErr)
		require.Equal(t, []int64{direct.ID}, permissionIDs(permissions))
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	permissions, err = identity.GetAccountPermissions(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, []int64{roleGrant.ID}, permissionIDs(permissions))

	require.NoError(t, identity.RemovePermissionFromAccount(ctx, account.ID, direct.ID))
	require.NoError(t, identity.DenyPermissionToAccount(ctx, account.ID, direct.ID), "a first denial stores false rather than the database default")
	permissions, err = identity.GetAccountDirectPermissions(ctx, account.ID)
	require.NoError(t, err)
	require.Empty(t, permissions)
	require.NoError(t, identity.RemovePermissionFromAccount(ctx, account.ID, direct.ID))
	permissions, err = identity.GetAccountDirectPermissions(otherCtx, account.ID)
	require.NoError(t, err)
	require.ElementsMatch(t, []int64{direct.ID, foreignGrant.ID}, permissionIDs(permissions), "school cleanup leaves other schools untouched")
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
