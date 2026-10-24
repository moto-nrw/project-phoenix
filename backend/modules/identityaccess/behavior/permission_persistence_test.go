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

// TestDeletePermissionRemovesStoredDirectGrants keeps the cleanup path for
// retired direct grants (#4485): deleting a permission still removes its rows
// until the table itself is dropped.
func TestDeletePermissionRemovesStoredDirectGrants(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	identity := roleAdministrationOf(t, setupAuthService(t, db))
	first := testpkg.CreateTestAccount(t, db, "direct-first")
	second := testpkg.CreateTestAccount(t, db, "direct-second")
	shared := testpkg.CreateTestPermission(t, db, "direct-shared", "direct-shared", "read")
	kept := testpkg.CreateTestPermission(t, db, "direct-kept", "direct-kept", "read")
	insert := func(accountID, permissionID int64) {
		t.Helper()
		_, err := db.NewRaw("INSERT INTO auth.account_permissions (account_id, permission_id, granted, tenant_id) VALUES (?, ?, TRUE, ?)",
			accountID, permissionID, testpkg.Tenant(t)).Exec(ctx)
		require.NoError(t, err)
	}
	stored := func(accountID, permissionID int64) int {
		t.Helper()
		var count int
		err := db.NewRaw("SELECT count(*) FROM auth.account_permissions WHERE account_id = ? AND permission_id = ? AND tenant_id = ?",
			accountID, permissionID, testpkg.Tenant(t)).Scan(ctx, &count)
		require.NoError(t, err)
		return count
	}
	insert(first.ID, shared.ID)
	insert(first.ID, kept.ID)
	insert(second.ID, shared.ID)

	require.NoError(t, identity.DeletePermission(ctx, shared.ID))
	require.Zero(t, stored(first.ID, shared.ID))
	require.Zero(t, stored(second.ID, shared.ID))
	require.Equal(t, 1, stored(first.ID, kept.ID))
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
