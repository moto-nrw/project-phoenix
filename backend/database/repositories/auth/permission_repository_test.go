package auth_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Setup Helpers
// ============================================================================

// ============================================================================
// CRUD Tests
// ============================================================================

func TestPermissionRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Permission
	ctx := testpkg.Ctx(t)

	t.Run("creates permission with valid data", func(t *testing.T) {
		uniqueName := fmt.Sprintf("test_permission_%d", time.Now().UnixNano())
		permission := &auth.Permission{
			Name:        uniqueName,
			Resource:    "test_resource",
			Action:      "read",
			Description: "Test permission",
		}

		err := repo.Create(ctx, permission)
		require.NoError(t, err)
		assert.NotZero(t, permission.ID)
		t.Cleanup(func() { require.NoError(t, repo.Delete(ctx, permission.ID)) })
	})

	t.Run("creates permission with different actions", func(t *testing.T) {
		uniqueName := fmt.Sprintf("test_write_permission_%d", time.Now().UnixNano())
		permission := &auth.Permission{
			Name:        uniqueName,
			Resource:    "test_resource",
			Action:      "write",
			Description: "Write permission",
		}

		err := repo.Create(ctx, permission)
		require.NoError(t, err)
		assert.NotZero(t, permission.ID)
		t.Cleanup(func() { require.NoError(t, repo.Delete(ctx, permission.ID)) })
	})
}

func TestPermissionRepository_FindByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Permission
	ctx := testpkg.Ctx(t)

	t.Run("finds existing permission", func(t *testing.T) {
		permission := testpkg.CreateTestPermission(t, db, "FindByID", "resource", "read")

		found, err := repo.FindByID(ctx, permission.ID)
		require.NoError(t, err)
		assert.Equal(t, permission.ID, found.ID)
		assert.Contains(t, found.Name, "FindByID")
	})

	t.Run("returns error for non-existent permission", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999))
		require.Error(t, err)
	})
}

func TestPermissionRepository_FindByName(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Permission
	ctx := testpkg.Ctx(t)

	t.Run("finds permission by exact name", func(t *testing.T) {
		permission := testpkg.CreateTestPermission(t, db, "FindByName", "resource", "read")

		found, err := repo.FindByName(ctx, permission.Name)
		require.NoError(t, err)
		assert.Equal(t, permission.ID, found.ID)
	})

	t.Run("returns error for non-existent name", func(t *testing.T) {
		_, err := repo.FindByName(ctx, "NonExistentPermission12345")
		require.Error(t, err)
	})
}

func TestPermissionRepository_Update(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Permission
	ctx := testpkg.Ctx(t)

	t.Run("updates permission description", func(t *testing.T) {
		permission := testpkg.CreateTestPermission(t, db, "Update", "resource", "read")

		permission.Description = "Updated description"
		err := repo.Update(ctx, permission)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, permission.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated description", found.Description)
	})
}

func TestPermissionRepository_Delete(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Permission
	ctx := testpkg.Ctx(t)

	t.Run("deletes existing permission", func(t *testing.T) {
		permission := testpkg.CreateTestPermission(t, db, "Delete", "resource", "read")

		err := repo.Delete(ctx, permission.ID)
		require.NoError(t, err)

		_, err = repo.FindByID(ctx, permission.ID)
		require.Error(t, err)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestPermissionRepository_List(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Permission
	ctx := testpkg.Ctx(t)

	t.Run("lists all permissions", func(t *testing.T) {
		testpkg.CreateTestPermission(t, db, "List", "resource", "read")

		permissions, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, permissions)
	})
}

func TestPermissionRepository_FindByRoleID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Permission
	ctx := testpkg.Ctx(t)

	t.Run("finds permissions assigned to role", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, db, "PermRole")
		permission := testpkg.CreateTestPermission(t, db, "ByRoleID", "resource", "read")

		// Assign permission to role
		_, err := db.ExecContext(ctx,
			"INSERT INTO auth.role_permissions (role_id, permission_id) VALUES (?, ?)",
			role.ID, permission.ID)
		require.NoError(t, err)

		// Find permissions
		permissions, err := repo.FindByRoleID(ctx, role.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, permissions)

		var found bool
		for _, p := range permissions {
			if p.ID == permission.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

func TestPermissionRepository_FindByAccountID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Permission
	ctx := testpkg.Ctx(t)

	t.Run("finds permissions for account via role", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "permacc")
		role := testpkg.CreateTestRole(t, db, "PermAccRole")
		permission := testpkg.CreateTestPermission(t, db, "ByAccountID", "resource", "read")

		// Assign role to account
		_, err := db.ExecContext(ctx,
			"INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)",
			account.ID, role.ID, testpkg.Tenant(t))
		require.NoError(t, err)

		// Assign permission to role
		_, err = db.ExecContext(ctx,
			"INSERT INTO auth.role_permissions (role_id, permission_id) VALUES (?, ?)",
			role.ID, permission.ID)
		require.NoError(t, err)

		// Find permissions (includes both direct and via role)
		permissions, err := repo.FindByAccountID(ctx, account.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, permissions)

		var found bool
		for _, p := range permissions {
			if p.ID == permission.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("returns empty for account with no permissions", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "noperms")

		permissions, err := repo.FindByAccountID(ctx, account.ID)
		require.NoError(t, err)
		assert.Empty(t, permissions)
	})
}

func TestPermissionRepository_FindDirectByAccountID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Permission
	ctx := testpkg.Ctx(t)

	t.Run("finds directly assigned permissions only", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "directperm")
		permission := testpkg.CreateTestPermission(t, db, "DirectByAccountID", "resource", "read")

		// Assign permission directly to account (granted=true)
		_, err := db.ExecContext(ctx,
			"INSERT INTO auth.account_permissions (account_id, permission_id, granted, tenant_id) VALUES (?, ?, ?, ?)",
			account.ID, permission.ID, true, testpkg.Tenant(t))
		require.NoError(t, err)

		// Find direct permissions
		permissions, err := repo.FindDirectByAccountID(ctx, account.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, permissions)

		var found bool
		for _, p := range permissions {
			if p.ID == permission.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

// ============================================================================
// Permission Assignment Tests
// ============================================================================

// Using direct DB access for reliable tests.

func TestPermissionRepository_AssignPermissionToRole(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Permission
	ctx := testpkg.Ctx(t)

	t.Run("assigns permission to role", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, db, "AssignPerm")
		permission := testpkg.CreateTestPermission(t, db, "AssignToRole", "resource", "read")

		err := repo.AssignPermissionToRole(ctx, role.ID, permission.ID)
		require.NoError(t, err)

		// Verify assignment
		permissions, err := repo.FindByRoleID(ctx, role.ID)
		require.NoError(t, err)
		assert.Len(t, permissions, 1)
	})
}

func TestPermissionRepository_RemovePermissionFromRole(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Permission
	ctx := testpkg.Ctx(t)

	t.Run("removes permission from role", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, db, "RemovePerm")
		permission := testpkg.CreateTestPermission(t, db, "RemoveFromRole", "resource", "read")

		// Assign permission to role directly
		_, err := db.ExecContext(ctx,
			"INSERT INTO auth.role_permissions (role_id, permission_id) VALUES (?, ?)",
			role.ID, permission.ID)
		require.NoError(t, err)

		// Remove permission
		err = repo.RemovePermissionFromRole(ctx, role.ID, permission.ID)
		require.NoError(t, err)

		// Verify removal
		permissions, err := repo.FindByRoleID(ctx, role.ID)
		require.NoError(t, err)
		assert.Empty(t, permissions)
	})
}
