package auth_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/auth"
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
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).RolePermission
	ctx := context.Background()

	t.Run("creates role permission mapping", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, db, "test_rp_create_role")
		permission := testpkg.CreateTestPermission(t, db, "TestRPCreate", "rp_create", "read")
		defer cleanupRoleRecords(t, db, role.ID)
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
		defer cleanupRoleRecords(t, db, role.ID)

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
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).RolePermission
	ctx := context.Background()

	t.Run("finds permissions by role ID", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, db, "test_rp_find_by_role")
		permission1 := testpkg.CreateTestPermission(t, db, "RPFindRole1", "rp_findrole1", "read")
		permission2 := testpkg.CreateTestPermission(t, db, "RPFindRole2", "rp_findrole2", "write")
		defer cleanupRoleRecords(t, db, role.ID)
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
		defer cleanupRoleRecords(t, db, role.ID)

		perms, err := repo.FindByRoleID(ctx, role.ID)
		require.NoError(t, err)
		assert.Empty(t, perms)
	})
}

func TestRolePermissionRepository_FindByPermissionID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).RolePermission
	ctx := context.Background()

	t.Run("finds roles by permission ID", func(t *testing.T) {
		role1 := testpkg.CreateTestRole(t, db, "test_rp_find_by_perm1")
		role2 := testpkg.CreateTestRole(t, db, "test_rp_find_by_perm2")
		permission := testpkg.CreateTestPermission(t, db, "RPFindPerm", "rp_findperm", "execute")
		defer cleanupRoleRecords(t, db, role1.ID, role2.ID)
		defer cleanupPermissionByID(t, db, permission.ID)

		// Create mappings
		rp1 := &auth.RolePermission{RoleID: role1.ID, PermissionID: permission.ID}
		rp2 := &auth.RolePermission{RoleID: role2.ID, PermissionID: permission.ID}
		require.NoError(t, repo.Create(ctx, rp1))
		require.NoError(t, repo.Create(ctx, rp2))

		// Find by permission ID
		rolePerms, err := repo.FindByPermissionID(ctx, permission.ID)
		require.NoError(t, err)
		assert.Len(t, rolePerms, 2)
	})

	t.Run("returns empty slice for permission with no roles", func(t *testing.T) {
		permission := testpkg.CreateTestPermission(t, db, "RPNoRoles", "rp_noroles", "admin")
		defer cleanupPermissionByID(t, db, permission.ID)

		rolePerms, err := repo.FindByPermissionID(ctx, permission.ID)
		require.NoError(t, err)
		assert.Empty(t, rolePerms)
	})
}

func TestRolePermissionRepository_FindByRoleAndPermission(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).RolePermission
	ctx := context.Background()

	t.Run("finds specific role-permission mapping", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, db, "test_rp_find_specific")
		permission := testpkg.CreateTestPermission(t, db, "RPFindSpecific", "rp_findspecific", "read")
		defer cleanupRoleRecords(t, db, role.ID)
		defer cleanupPermissionByID(t, db, permission.ID)

		// Create mapping
		rp := &auth.RolePermission{RoleID: role.ID, PermissionID: permission.ID}
		require.NoError(t, repo.Create(ctx, rp))

		// Find the specific mapping
		found, err := repo.FindByRoleAndPermission(ctx, role.ID, permission.ID)
		require.NoError(t, err)
		assert.Equal(t, role.ID, found.RoleID)
		assert.Equal(t, permission.ID, found.PermissionID)
	})

	t.Run("returns error for non-existent mapping", func(t *testing.T) {
		_, err := repo.FindByRoleAndPermission(ctx, 999999, 999999)
		require.Error(t, err)
	})
}

func TestRolePermissionRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).RolePermission
	ctx := context.Background()

	t.Run("updates role permission mapping", func(t *testing.T) {
		role1 := testpkg.CreateTestRole(t, db, "test_rp_update_role1")
		role2 := testpkg.CreateTestRole(t, db, "test_rp_update_role2")
		permission := testpkg.CreateTestPermission(t, db, "RPUpdate", "rp_update", "write")
		defer cleanupRoleRecords(t, db, role1.ID, role2.ID)
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

func TestRolePermissionRepository_DeleteByRoleAndPermission(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).RolePermission
	ctx := context.Background()

	t.Run("deletes specific role-permission mapping", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, db, "test_rp_delete_specific")
		permission := testpkg.CreateTestPermission(t, db, "RPDeleteSpecific", "rp_deletespecific", "delete")
		defer cleanupRoleRecords(t, db, role.ID)
		defer cleanupPermissionByID(t, db, permission.ID)

		// Create mapping
		rp := &auth.RolePermission{RoleID: role.ID, PermissionID: permission.ID}
		require.NoError(t, repo.Create(ctx, rp))

		// Delete it
		err := repo.DeleteByRoleAndPermission(ctx, role.ID, permission.ID)
		require.NoError(t, err)

		// Verify deleted
		_, err = repo.FindByRoleAndPermission(ctx, role.ID, permission.ID)
		require.Error(t, err)
	})

	t.Run("does not error when deleting non-existent mapping", func(t *testing.T) {
		err := repo.DeleteByRoleAndPermission(ctx, 999999, 999999)
		require.NoError(t, err)
	})
}

func TestRolePermissionRepository_DeleteByRoleID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).RolePermission
	ctx := context.Background()

	t.Run("deletes all permissions for a role", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, db, "test_rp_delete_by_role")
		permission1 := testpkg.CreateTestPermission(t, db, "RPDeleteByRole1", "rp_deletebyrole1", "read")
		permission2 := testpkg.CreateTestPermission(t, db, "RPDeleteByRole2", "rp_deletebyrole2", "write")
		defer cleanupRoleRecords(t, db, role.ID)
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
		defer cleanupRoleRecords(t, db, role.ID)

		err := repo.DeleteByRoleID(ctx, role.ID)
		require.NoError(t, err)
	})
}

func TestRolePermissionRepository_DeleteByPermissionID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).RolePermission
	ctx := context.Background()

	t.Run("deletes all roles for a permission", func(t *testing.T) {
		role1 := testpkg.CreateTestRole(t, db, "test_rp_delete_by_perm1")
		role2 := testpkg.CreateTestRole(t, db, "test_rp_delete_by_perm2")
		permission := testpkg.CreateTestPermission(t, db, "RPDeleteByPerm", "rp_deletebyperm", "admin")
		defer cleanupRoleRecords(t, db, role1.ID, role2.ID)
		defer cleanupPermissionByID(t, db, permission.ID)

		// Create mappings
		rp1 := &auth.RolePermission{RoleID: role1.ID, PermissionID: permission.ID}
		rp2 := &auth.RolePermission{RoleID: role2.ID, PermissionID: permission.ID}
		require.NoError(t, repo.Create(ctx, rp1))
		require.NoError(t, repo.Create(ctx, rp2))

		// Verify mappings exist
		rolePerms, err := repo.FindByPermissionID(ctx, permission.ID)
		require.NoError(t, err)
		assert.Len(t, rolePerms, 2)

		// Delete all by permission ID
		err = repo.DeleteByPermissionID(ctx, permission.ID)
		require.NoError(t, err)

		// Verify all deleted
		rolePerms, err = repo.FindByPermissionID(ctx, permission.ID)
		require.NoError(t, err)
		assert.Empty(t, rolePerms)
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
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).RolePermission
	ctx := context.Background()

	t.Run("lists all role permissions", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, db, "test_rp_list")
		permission := testpkg.CreateTestPermission(t, db, "RPList", "rp_list", "read")
		defer cleanupRoleRecords(t, db, role.ID)
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
		defer cleanupRoleRecords(t, db, role.ID)
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
		defer cleanupRoleRecords(t, db, role.ID)
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

func TestRolePermissionRepository_FindRolePermissionsWithDetails(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).RolePermission
	ctx := context.Background()

	t.Run("finds role permissions with role and permission details", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, db, "test_rp_with_details")
		permission := testpkg.CreateTestPermission(t, db, "RPWithDetails", "rp_withdetails", "read")
		defer cleanupRoleRecords(t, db, role.ID)
		defer cleanupPermissionByID(t, db, permission.ID)

		rp := &auth.RolePermission{RoleID: role.ID, PermissionID: permission.ID}
		require.NoError(t, repo.Create(ctx, rp))

		perms, err := repo.FindRolePermissionsWithDetails(ctx, map[string]any{
			"role_id": role.ID,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, perms)

		// Check that relations are loaded
		found := perms[0]
		assert.Equal(t, role.ID, found.RoleID)
		assert.Equal(t, permission.ID, found.PermissionID)
		// Relations should be loaded via Relation()
		assert.NotNil(t, found.Role)
		assert.NotNil(t, found.Permission)
	})

	t.Run("filters by permission_id with details", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, db, "test_rp_details_by_perm")
		permission := testpkg.CreateTestPermission(t, db, "RPDetailsByPerm", "rp_detailsbyperm", "manage")
		defer cleanupRoleRecords(t, db, role.ID)
		defer cleanupPermissionByID(t, db, permission.ID)

		rp := &auth.RolePermission{RoleID: role.ID, PermissionID: permission.ID}
		require.NoError(t, repo.Create(ctx, rp))

		perms, err := repo.FindRolePermissionsWithDetails(ctx, map[string]any{
			"permission_id": permission.ID,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, perms)
	})
}
