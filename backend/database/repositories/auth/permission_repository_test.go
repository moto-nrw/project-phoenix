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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Permission
	ctx := testpkg.TenantContext(1)

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

		testpkg.CleanupPermissionRecords(t, db, permission.ID)
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

		testpkg.CleanupPermissionRecords(t, db, permission.ID)
	})
}

func TestPermissionRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Permission
	ctx := testpkg.TenantContext(1)

	t.Run("finds existing permission", func(t *testing.T) {
		permission := testpkg.CreateTestPermission(t, db, "FindByID", "resource", "read")
		defer testpkg.CleanupPermissionRecords(t, db, permission.ID)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Permission
	ctx := testpkg.TenantContext(1)

	t.Run("finds permission by exact name", func(t *testing.T) {
		permission := testpkg.CreateTestPermission(t, db, "FindByName", "resource", "read")
		defer testpkg.CleanupPermissionRecords(t, db, permission.ID)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Permission
	ctx := testpkg.TenantContext(1)

	t.Run("updates permission description", func(t *testing.T) {
		permission := testpkg.CreateTestPermission(t, db, "Update", "resource", "read")
		defer testpkg.CleanupPermissionRecords(t, db, permission.ID)

		permission.Description = "Updated description"
		err := repo.Update(ctx, permission)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, permission.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated description", found.Description)
	})
}

func TestPermissionRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Permission
	ctx := testpkg.TenantContext(1)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Permission
	ctx := testpkg.TenantContext(1)

	t.Run("lists all permissions", func(t *testing.T) {
		permission := testpkg.CreateTestPermission(t, db, "List", "resource", "read")
		defer testpkg.CleanupPermissionRecords(t, db, permission.ID)

		permissions, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, permissions)
	})
}

func TestPermissionRepository_FindByRoleID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Permission
	ctx := testpkg.TenantContext(1)

	t.Run("finds permissions assigned to role", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, db, "PermRole")
		permission := testpkg.CreateTestPermission(t, db, "ByRoleID", "resource", "read")
		defer testpkg.CleanupRoleRecords(t, db, role.ID)
		defer testpkg.CleanupPermissionRecords(t, db, permission.ID)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Permission
	ctx := testpkg.TenantContext(1)

	t.Run("finds permissions for account via role", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "permacc")
		role := testpkg.CreateTestRole(t, db, "PermAccRole")
		permission := testpkg.CreateTestPermission(t, db, "ByAccountID", "resource", "read")
		defer cleanupAccountRecords(t, db, account.ID)
		defer testpkg.CleanupRoleRecords(t, db, role.ID)
		defer testpkg.CleanupPermissionRecords(t, db, permission.ID)

		// Assign role to account
		_, err := db.ExecContext(ctx,
			"INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, 1)",
			account.ID, role.ID)
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
		defer cleanupAccountRecords(t, db, account.ID)

		permissions, err := repo.FindByAccountID(ctx, account.ID)
		require.NoError(t, err)
		assert.Empty(t, permissions)
	})
}

func TestPermissionRepository_FindDirectByAccountID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Permission
	ctx := testpkg.TenantContext(1)

	t.Run("finds directly assigned permissions only", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "directperm")
		permission := testpkg.CreateTestPermission(t, db, "DirectByAccountID", "resource", "read")
		defer cleanupAccountRecords(t, db, account.ID)
		defer testpkg.CleanupPermissionRecords(t, db, permission.ID)

		// Assign permission directly to account (granted=true)
		_, err := db.ExecContext(ctx,
			"INSERT INTO auth.account_permissions (account_id, permission_id, granted, tenant_id) VALUES (?, ?, ?, 1)",
			account.ID, permission.ID, true)
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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Permission
	ctx := testpkg.TenantContext(1)

	t.Run("assigns permission to role", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, db, "AssignPerm")
		permission := testpkg.CreateTestPermission(t, db, "AssignToRole", "resource", "read")
		defer testpkg.CleanupRoleRecords(t, db, role.ID)
		defer testpkg.CleanupPermissionRecords(t, db, permission.ID)

		err := repo.AssignPermissionToRole(ctx, role.ID, permission.ID)
		require.NoError(t, err)

		// Verify assignment
		permissions, err := repo.FindByRoleID(ctx, role.ID)
		require.NoError(t, err)
		assert.Len(t, permissions, 1)
	})
}

func TestPermissionRepository_RemovePermissionFromRole(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Permission
	ctx := testpkg.TenantContext(1)

	t.Run("removes permission from role", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, db, "RemovePerm")
		permission := testpkg.CreateTestPermission(t, db, "RemoveFromRole", "resource", "read")
		defer testpkg.CleanupRoleRecords(t, db, role.ID)
		defer testpkg.CleanupPermissionRecords(t, db, permission.ID)

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
