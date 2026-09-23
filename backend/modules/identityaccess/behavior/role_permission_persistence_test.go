package behavior_test

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestRolePermissionPersistence(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	rbac := roleAdministrationOf(t, setupAuthService(t, db))
	ctx := testpkg.Ctx(t)
	first := testpkg.CreateTestRole(t, db, "permission-first")
	second := testpkg.CreateTestRole(t, db, "permission-second")
	shared := testpkg.CreateTestPermission(t, db, "permission-shared", "mapping", "read")
	kept := testpkg.CreateTestPermission(t, db, "permission-kept", "mapping", "write")

	assertPermissions := func(roleID int64, ids ...int64) {
		t.Helper()
		permissions, err := rbac.GetRolePermissions(ctx, roleID)
		require.NoError(t, err)
		actual := make([]int64, 0, len(permissions))
		for _, permission := range permissions {
			actual = append(actual, permission.ID)
		}
		require.ElementsMatch(t, ids, actual)
	}

	assertPermissions(first.ID)
	require.Error(t, rbac.AssignPermissionToRole(ctx, 0, shared.ID))
	require.Error(t, rbac.AssignPermissionToRole(ctx, first.ID, 0))
	assertPermissions(first.ID)
	require.NoError(t, rbac.AssignPermissionToRole(ctx, first.ID, shared.ID))
	require.NoError(t, rbac.AssignPermissionToRole(ctx, first.ID, shared.ID))
	require.NoError(t, rbac.AssignPermissionToRole(ctx, first.ID, kept.ID))
	require.NoError(t, rbac.AssignPermissionToRole(ctx, second.ID, shared.ID))
	assertPermissions(first.ID, shared.ID, kept.ID)
	assertPermissions(second.ID, shared.ID)

	// Replacement changes only the selected role, and an invalid selection
	// leaves its existing grants intact.
	require.Error(t, rbac.ReplaceRolePermissions(ctx, first.ID, []int64{0}))
	assertPermissions(first.ID, shared.ID, kept.ID)
	require.NoError(t, rbac.ReplaceRolePermissions(ctx, first.ID, []int64{kept.ID}))
	assertPermissions(first.ID, kept.ID)
	assertPermissions(second.ID, shared.ID)
	require.NoError(t, rbac.AssignPermissionToRole(ctx, first.ID, shared.ID))

	// Deleting a permission removes every mapping without removing the
	// roles or their other permissions.
	require.NoError(t, rbac.DeletePermission(ctx, shared.ID))
	assertPermissions(first.ID, kept.ID)
	assertPermissions(second.ID)
	_, err := rbac.GetRole(ctx, second.ID)
	require.NoError(t, err)

	require.NoError(t, rbac.AssignPermissionToRole(ctx, second.ID, kept.ID))
	require.NoError(t, rbac.DeleteRole(ctx, first.ID))
	assertPermissions(first.ID)
	assertPermissions(second.ID, kept.ID)
	_, err = rbac.GetPermission(ctx, kept.ID)
	require.NoError(t, err)
	require.NoError(t, rbac.RemovePermissionFromRole(ctx, second.ID, kept.ID))
	assertPermissions(second.ID)
}
