package auth_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// cleanupRolePermission removes a role-permission mapping from the database.
func cleanupRolePermission(t *testing.T, db *bun.DB, roleID, permissionID int64) {
	t.Helper()
	_, _ = db.NewDelete().
		Model((*auth.RolePermission)(nil)).
		TableExpr("auth.role_permissions").
		Where("role_id = ? AND permission_id = ?", roleID, permissionID).
		Exec(context.Background())
}

// ============================================================================
// RolePermissionRepository CRUD Tests
// ============================================================================

func TestRolePermissionRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).RolePermission
	ctx := testpkg.Ctx(t)

	t.Run("creates role permission mapping", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, db, "test_rp_create_role")
		permission := testpkg.CreateTestPermission(t, db, "TestRPCreate", "rp_create", "read")
		defer testpkg.CleanupRoleRecords(t, db, role.ID)
		defer cleanupPermissionByID(t, db, permission.ID)

		rp := &auth.RolePermission{
			RoleID:       role.ID,
			PermissionID: permission.ID,
		}

		err := repo.Create(ctx, rp)
		require.NoError(t, err)
		assert.NotZero(t, rp.ID)

		defer cleanupRolePermission(t, db, role.ID, permission.ID)
	})

	t.Run("rejects nil role permission", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("rejects invalid role permission - missing role ID", func(t *testing.T) {
		permission := testpkg.CreateTestPermission(t, db, "TestRPNoRole", "rp_norole", "read")
		defer cleanupPermissionByID(t, db, permission.ID)

		rp := &auth.RolePermission{
			RoleID:       0, // Invalid
			PermissionID: permission.ID,
		}

		err := repo.Create(ctx, rp)
		require.Error(t, err)
	})

	t.Run("rejects invalid role permission - missing permission ID", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, db, "test_rp_noperm_role")
		defer testpkg.CleanupRoleRecords(t, db, role.ID)

		rp := &auth.RolePermission{
			RoleID:       role.ID,
			PermissionID: 0, // Invalid
		}

		err := repo.Create(ctx, rp)
		require.Error(t, err)
	})
}

func TestRolePermissionRepository_FindByRoleID(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).RolePermission
	ctx := testpkg.Ctx(t)

	t.Run("finds permissions by role ID", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, db, "test_rp_find_by_role")
		permission1 := testpkg.CreateTestPermission(t, db, "RPFindRole1", "rp_findrole1", "read")
		permission2 := testpkg.CreateTestPermission(t, db, "RPFindRole2", "rp_findrole2", "write")
		defer testpkg.CleanupRoleRecords(t, db, role.ID)
		defer cleanupPermissionByID(t, db, permission1.ID)
		defer cleanupPermissionByID(t, db, permission2.ID)

		// Create mappings
		rp1 := &auth.RolePermission{RoleID: role.ID, PermissionID: permission1.ID}
		rp2 := &auth.RolePermission{RoleID: role.ID, PermissionID: permission2.ID}
		require.NoError(t, repo.Create(ctx, rp1))
		require.NoError(t, repo.Create(ctx, rp2))

		// Find by role ID
		perms, err := repo.FindByRoleID(ctx, role.ID)
		require.NoError(t, err)
		assert.Len(t, perms, 2)
	})

	t.Run("returns empty slice for role with no permissions", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, db, "test_rp_empty_role")
		defer testpkg.CleanupRoleRecords(t, db, role.ID)

		perms, err := repo.FindByRoleID(ctx, role.ID)
		require.NoError(t, err)
		assert.Empty(t, perms)
	})
}

func TestRolePermissionRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).RolePermission
	ctx := testpkg.Ctx(t)

	t.Run("updates role permission mapping", func(t *testing.T) {
		role1 := testpkg.CreateTestRole(t, db, "test_rp_update_role1")
		role2 := testpkg.CreateTestRole(t, db, "test_rp_update_role2")
		permission := testpkg.CreateTestPermission(t, db, "RPUpdate", "rp_update", "write")
		defer testpkg.CleanupRoleRecords(t, db, role1.ID, role2.ID)
		defer cleanupPermissionByID(t, db, permission.ID)

		// Create initial mapping
		rp := &auth.RolePermission{RoleID: role1.ID, PermissionID: permission.ID}
		require.NoError(t, repo.Create(ctx, rp))
		defer cleanupRolePermission(t, db, role1.ID, permission.ID)
		defer cleanupRolePermission(t, db, role2.ID, permission.ID)

		// Update to different role
		rp.RoleID = role2.ID
		err := repo.Update(ctx, rp)
		require.NoError(t, err)
	})

	t.Run("rejects nil role permission", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("rejects invalid update - missing role ID", func(t *testing.T) {
		permission := testpkg.CreateTestPermission(t, db, "RPUpdateInvalid", "rp_updateinvalid", "read")
		defer cleanupPermissionByID(t, db, permission.ID)

		rp := &auth.RolePermission{
			RoleID:       0,
			PermissionID: permission.ID,
		}
		rp.ID = 1 // Fake ID for update

		err := repo.Update(ctx, rp)
		require.Error(t, err)
	})
}

func TestRolePermissionRepository_DeleteByRoleID(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).RolePermission
	ctx := testpkg.Ctx(t)

	t.Run("deletes all permissions for a role", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, db, "test_rp_delete_by_role")
		permission1 := testpkg.CreateTestPermission(t, db, "RPDeleteByRole1", "rp_deletebyrole1", "read")
		permission2 := testpkg.CreateTestPermission(t, db, "RPDeleteByRole2", "rp_deletebyrole2", "write")
		defer testpkg.CleanupRoleRecords(t, db, role.ID)
		defer cleanupPermissionByID(t, db, permission1.ID)
		defer cleanupPermissionByID(t, db, permission2.ID)

		// Create mappings
		rp1 := &auth.RolePermission{RoleID: role.ID, PermissionID: permission1.ID}
		rp2 := &auth.RolePermission{RoleID: role.ID, PermissionID: permission2.ID}
		require.NoError(t, repo.Create(ctx, rp1))
		require.NoError(t, repo.Create(ctx, rp2))

		// Verify mappings exist
		perms, err := repo.FindByRoleID(ctx, role.ID)
		require.NoError(t, err)
		assert.Len(t, perms, 2)

		// Delete all by role ID
		err = repo.DeleteByRoleID(ctx, role.ID)
		require.NoError(t, err)

		// Verify all deleted
		perms, err = repo.FindByRoleID(ctx, role.ID)
		require.NoError(t, err)
		assert.Empty(t, perms)
	})

	t.Run("does not error when role has no permissions", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, db, "test_rp_delete_empty_role")
		defer testpkg.CleanupRoleRecords(t, db, role.ID)

		err := repo.DeleteByRoleID(ctx, role.ID)
		require.NoError(t, err)
	})
}

func TestRolePermissionRepository_DeleteByPermissionID(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).RolePermission
	ctx := testpkg.Ctx(t)

	t.Run("deletes all roles for a permission", func(t *testing.T) {
		role1 := testpkg.CreateTestRole(t, db, "test_rp_delete_by_perm1")
		role2 := testpkg.CreateTestRole(t, db, "test_rp_delete_by_perm2")
		permission := testpkg.CreateTestPermission(t, db, "RPDeleteByPerm", "rp_deletebyperm", "admin")
		defer testpkg.CleanupRoleRecords(t, db, role1.ID, role2.ID)
		defer cleanupPermissionByID(t, db, permission.ID)

		// Create mappings
		rp1 := &auth.RolePermission{RoleID: role1.ID, PermissionID: permission.ID}
		rp2 := &auth.RolePermission{RoleID: role2.ID, PermissionID: permission.ID}
		require.NoError(t, repo.Create(ctx, rp1))
		require.NoError(t, repo.Create(ctx, rp2))

		// Delete all by permission ID
		err := repo.DeleteByPermissionID(ctx, permission.ID)
		require.NoError(t, err)

		// Verify all deleted via the live per-role lookup
		for _, roleID := range []int64{role1.ID, role2.ID} {
			rolePerms, err := repo.FindByRoleID(ctx, roleID)
			require.NoError(t, err)
			for _, rp := range rolePerms {
				assert.NotEqual(t, permission.ID, rp.PermissionID,
					"mapping for permission %d must be deleted", permission.ID)
			}
		}
	})

	t.Run("does not error when permission has no roles", func(t *testing.T) {
		permission := testpkg.CreateTestPermission(t, db, "RPDeleteEmptyPerm", "rp_deleteemptyperm", "read")
		defer cleanupPermissionByID(t, db, permission.ID)

		err := repo.DeleteByPermissionID(ctx, permission.ID)
		require.NoError(t, err)
	})
}

func TestRolePermissionRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).RolePermission
	ctx := testpkg.Ctx(t)

	t.Run("lists all role permissions", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, db, "test_rp_list")
		permission := testpkg.CreateTestPermission(t, db, "RPList", "rp_list", "read")
		defer testpkg.CleanupRoleRecords(t, db, role.ID)
		defer cleanupPermissionByID(t, db, permission.ID)

		rp := &auth.RolePermission{RoleID: role.ID, PermissionID: permission.ID}
		require.NoError(t, repo.Create(ctx, rp))

		perms, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, perms)
	})

	t.Run("filters by role_id", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, db, "test_rp_list_by_role")
		permission := testpkg.CreateTestPermission(t, db, "RPListByRole", "rp_listbyrole", "write")
		defer testpkg.CleanupRoleRecords(t, db, role.ID)
		defer cleanupPermissionByID(t, db, permission.ID)

		rp := &auth.RolePermission{RoleID: role.ID, PermissionID: permission.ID}
		require.NoError(t, repo.Create(ctx, rp))

		perms, err := repo.List(ctx, map[string]any{
			"role_id": role.ID,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, perms)
		for _, p := range perms {
			assert.Equal(t, role.ID, p.RoleID)
		}
	})

	t.Run("filters by permission_id", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, db, "test_rp_list_by_perm")
		permission := testpkg.CreateTestPermission(t, db, "RPListByPerm", "rp_listbyperm", "execute")
		defer testpkg.CleanupRoleRecords(t, db, role.ID)
		defer cleanupPermissionByID(t, db, permission.ID)

		rp := &auth.RolePermission{RoleID: role.ID, PermissionID: permission.ID}
		require.NoError(t, repo.Create(ctx, rp))

		perms, err := repo.List(ctx, map[string]any{
			"permission_id": permission.ID,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, perms)
		for _, p := range perms {
			assert.Equal(t, permission.ID, p.PermissionID)
		}
	})
}
