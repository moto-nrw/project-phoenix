package auth_test

import (
	"context"
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
// CRUD Tests
// ============================================================================

func TestRoleRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Role
	ctx := testpkg.Ctx(t)

	t.Run("creates role with valid data", func(t *testing.T) {
		uniqueName := fmt.Sprintf("TestRole-%d", time.Now().UnixNano())
		role := &auth.Role{
			Name:        uniqueName,
			Description: "Test role description",
			IsSystem:    false,
		}

		err := repo.Create(ctx, role)
		require.NoError(t, err)
		assert.NotZero(t, role.ID)
	})

	t.Run("creates system role", func(t *testing.T) {
		uniqueName := fmt.Sprintf("SystemRole-%d", time.Now().UnixNano())
		role := &auth.Role{
			Name:        uniqueName,
			Description: "System role",
			IsSystem:    true,
		}

		err := repo.Create(ctx, role)
		require.NoError(t, err)
		assert.True(t, role.IsSystem)
	})
}

func TestRoleRepository_FindByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Role
	ctx := testpkg.Ctx(t)

	t.Run("finds existing role", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, db, "FindByID")

		found, err := repo.FindByID(ctx, role.ID)
		require.NoError(t, err)
		assert.Equal(t, role.ID, found.ID)
		assert.Contains(t, found.Name, "FindByID")
	})

	t.Run("returns error for non-existent role", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999))
		require.Error(t, err)
	})
}

func TestRoleRepository_FindByName(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Role
	ctx := testpkg.Ctx(t)

	t.Run("finds role by exact name", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, db, "FindByName")

		found, err := repo.FindByName(ctx, role.Name)
		require.NoError(t, err)
		assert.Equal(t, role.ID, found.ID)
	})

	t.Run("returns error for non-existent name", func(t *testing.T) {
		_, err := repo.FindByName(ctx, "NonExistentRole12345")
		require.Error(t, err)
	})
}

func TestRoleRepository_Update(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Role
	ctx := testpkg.Ctx(t)

	t.Run("updates role description", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, db, "Update")

		role.Description = "Updated description"
		err := repo.Update(ctx, role)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, role.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated description", found.Description)
	})
}

func TestRoleRepository_Delete(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Role
	ctx := testpkg.Ctx(t)

	t.Run("deletes existing role", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, db, "Delete")

		err := repo.Delete(ctx, role.ID)
		require.NoError(t, err)

		_, err = repo.FindByID(ctx, role.ID)
		require.Error(t, err)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestRoleRepository_List(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Role
	ctx := testpkg.Ctx(t)

	t.Run("lists all roles", func(t *testing.T) {
		testpkg.CreateTestRole(t, db, "List")

		roles, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, roles)
	})
}

func TestRoleRepository_FindByAccountID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Role
	ctx := testpkg.Ctx(t)

	t.Run("finds roles assigned to account", func(t *testing.T) {
		// Create account and role
		account := testpkg.CreateTestAccount(t, db, "roletest")
		role := testpkg.CreateTestRole(t, db, "AccountRole")

		// Assign role to account using direct DB insert (repo method deprecated)
		_, err := db.ExecContext(ctx,
			"INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)",
			account.ID, role.ID, testpkg.Tenant(t))
		require.NoError(t, err)

		// Find roles
		roles, err := repo.FindByAccountID(ctx, account.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, roles)

		var found bool
		for _, r := range roles {
			if r.ID == role.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("returns empty for account with no roles", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "noroles")

		roles, err := repo.FindByAccountID(ctx, account.ID)
		require.NoError(t, err)
		assert.Empty(t, roles)
	})

	t.Run("respects tenant scoping", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "tenantroles")
		roleTenant1 := testpkg.CreateTestRole(t, db, fmt.Sprintf("TenantOne_%d", time.Now().UnixNano()))
		roleTenant2 := testpkg.CreateTestRole(t, db, fmt.Sprintf("TenantTwo_%d", time.Now().UnixNano()))

		otherTenantID := testpkg.UniqueTestTenantID(t)
		testpkg.EnsureTestTenant(t, db, otherTenantID)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
		testpkg.EnsureAccountTenant(t, db, account.ID, otherTenantID)

		_, err := db.ExecContext(context.Background(),
			"INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?), (?, ?, ?)",
			account.ID, roleTenant1.ID, testpkg.Tenant(t), account.ID, roleTenant2.ID, otherTenantID)
		require.NoError(t, err)

		rolesTenant1, err := repo.FindByAccountID(testpkg.Ctx(t), account.ID)
		require.NoError(t, err)
		require.Len(t, rolesTenant1, 1)
		assert.Equal(t, roleTenant1.ID, rolesTenant1[0].ID)

		rolesTenant2, err := repo.FindByAccountID(testpkg.TenantContext(otherTenantID), account.ID)
		require.NoError(t, err)
		require.Len(t, rolesTenant2, 1)
		assert.Equal(t, roleTenant2.ID, rolesTenant2[0].ID)
	})
}

// ============================================================================
// Role Assignment Tests
// ============================================================================

// NOTE: AssignRoleToAccount and RemoveRoleFromAccount are deprecated.
// Use AccountRoleRepository.Create and AccountRoleRepository.Delete instead.
// These tests verify the deprecated methods return appropriate errors.

func TestRoleRepository_AssignRoleToAccount(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Role
	ctx := testpkg.Ctx(t)

	t.Run("deprecated method returns error", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "assign")
		role := testpkg.CreateTestRole(t, db, "Assign")

		err := repo.AssignRoleToAccount(ctx, account.ID, role.ID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "deprecated")
	})
}

func TestRoleRepository_RemoveRoleFromAccount(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Role
	ctx := testpkg.Ctx(t)

	t.Run("deprecated method returns error", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "remove")
		role := testpkg.CreateTestRole(t, db, "Remove")

		err := repo.RemoveRoleFromAccount(ctx, account.ID, role.ID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "deprecated")
	})
}

// ============================================================================
// Batch Role Name Loading Tests
// ============================================================================

func TestRoleRepository_FindRoleNamesByAccountIDs(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Role
	ctx := testpkg.Ctx(t)

	t.Run("returns role names for multiple accounts", func(t *testing.T) {
		account1 := testpkg.CreateTestAccount(t, db, "batch_role_1")
		account2 := testpkg.CreateTestAccount(t, db, "batch_role_2")
		role1 := testpkg.CreateTestRole(t, db, fmt.Sprintf("BatchAdmin_%d", time.Now().UnixNano()))
		role2 := testpkg.CreateTestRole(t, db, fmt.Sprintf("BatchUser_%d", time.Now().UnixNano()))

		// Assign roles
		_, err := db.ExecContext(ctx,
			"INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?), (?, ?, ?)",
			account1.ID, role1.ID, testpkg.Tenant(t), account2.ID, role2.ID, testpkg.Tenant(t))
		require.NoError(t, err)

		result, err := repo.FindRoleNamesByAccountIDs(ctx, []int64{account1.ID, account2.ID})
		require.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, role1.Name, result[account1.ID])
		assert.Equal(t, role2.Name, result[account2.ID])
	})

	t.Run("returns empty map for empty input", func(t *testing.T) {
		result, err := repo.FindRoleNamesByAccountIDs(ctx, []int64{})
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("returns empty map for accounts with no roles", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "batch_norole")

		result, err := repo.FindRoleNamesByAccountIDs(ctx, []int64{account.ID})
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("keeps first role when account has multiple roles", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "batch_multi")
		role1 := testpkg.CreateTestRole(t, db, fmt.Sprintf("MultiFirst_%d", time.Now().UnixNano()))
		role2 := testpkg.CreateTestRole(t, db, fmt.Sprintf("MultiSecond_%d", time.Now().UnixNano()))

		// Assign two roles — first inserted should be returned (ORDER BY created_at ASC)
		_, err := db.ExecContext(ctx,
			"INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)",
			account.ID, role1.ID, testpkg.Tenant(t))
		require.NoError(t, err)
		_, err = db.ExecContext(ctx,
			"INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)",
			account.ID, role2.ID, testpkg.Tenant(t))
		require.NoError(t, err)

		result, err := repo.FindRoleNamesByAccountIDs(ctx, []int64{account.ID})
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, role1.Name, result[account.ID])
	})

	t.Run("respects tenant scoping", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "batch_tenant")
		role := testpkg.CreateTestRole(t, db, fmt.Sprintf("TenantRole_%d", time.Now().UnixNano()))

		// Assign role to tenant 1
		_, err := db.ExecContext(ctx,
			"INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)",
			account.ID, role.ID, testpkg.Tenant(t))
		require.NoError(t, err)

		// Query with tenant 1 context — should find it
		result, err := repo.FindRoleNamesByAccountIDs(ctx, []int64{account.ID})
		require.NoError(t, err)
		assert.Equal(t, role.Name, result[account.ID])

		// Query from another tenant — should not find it
		ctx2 := testpkg.TenantContext(testpkg.UniqueTestTenantID(t))
		result2, err := repo.FindRoleNamesByAccountIDs(ctx2, []int64{account.ID})
		require.NoError(t, err)
		assert.Empty(t, result2)
	})
}
