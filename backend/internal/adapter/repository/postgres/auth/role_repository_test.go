package auth_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Setup Helpers
// ============================================================================

// cleanupRoleRecords removes roles directly
func cleanupRoleRecords(t *testing.T, db *bun.DB, roleIDs ...int64) {
	t.Helper()
	if len(roleIDs) == 0 {
		return
	}

	ctx := context.Background()

	// First remove any role-permission mappings
	_, _ = db.NewDelete().
		TableExpr("auth.role_permissions").
		Where("role_id IN (?)", bun.In(roleIDs)).
		Exec(ctx)

	// Then remove any account-role mappings
	_, _ = db.NewDelete().
		TableExpr("auth.account_roles").
		Where("role_id IN (?)", bun.In(roleIDs)).
		Exec(ctx)

	// Finally remove the roles
	_, err := db.NewDelete().
		TableExpr("auth.roles").
		Where("id IN (?)", bun.In(roleIDs)).
		Exec(ctx)
	if err != nil {
		t.Logf("Warning: failed to cleanup roles: %v", err)
	}
}

// cleanupAccountRecords removes accounts directly
func cleanupAccountRecords(t *testing.T, db *bun.DB, accountIDs ...int64) {
	t.Helper()
	if len(accountIDs) == 0 {
		return
	}

	ctx := context.Background()

	// Remove account-role mappings first
	_, _ = db.NewDelete().
		TableExpr("auth.account_roles").
		Where("account_id IN (?)", bun.In(accountIDs)).
		Exec(ctx)

	// Remove account-permission mappings
	_, _ = db.NewDelete().
		TableExpr("auth.account_permissions").
		Where("account_id IN (?)", bun.In(accountIDs)).
		Exec(ctx)

	// Remove tokens
	_, _ = db.NewDelete().
		TableExpr("auth.tokens").
		Where("account_id IN (?)", bun.In(accountIDs)).
		Exec(ctx)

	// Finally remove accounts
	_, err := db.NewDelete().
		TableExpr("auth.accounts").
		Where("id IN (?)", bun.In(accountIDs)).
		Exec(ctx)
	if err != nil {
		t.Logf("Warning: failed to cleanup accounts: %v", err)
	}
}

// ============================================================================
// CRUD Tests
// ============================================================================

func TestRoleRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Role
	ctx := context.Background()

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

		cleanupRoleRecords(t, db, role.ID)
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

		cleanupRoleRecords(t, db, role.ID)
	})
}

func TestRoleRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Role
	ctx := context.Background()

	t.Run("finds existing role", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, db, "FindByID")
		defer cleanupRoleRecords(t, db, role.ID)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Role
	ctx := context.Background()

	t.Run("finds role by exact name", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, db, "FindByName")
		defer cleanupRoleRecords(t, db, role.ID)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Role
	ctx := context.Background()

	t.Run("updates role description", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, db, "Update")
		defer cleanupRoleRecords(t, db, role.ID)

		role.Description = "Updated description"
		err := repo.Update(ctx, role)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, role.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated description", found.Description)
	})
}

func TestRoleRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Role
	ctx := context.Background()

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Role
	ctx := context.Background()

	t.Run("lists all roles", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, db, "List")
		defer cleanupRoleRecords(t, db, role.ID)

		roles, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, roles)
	})
}

func TestRoleRepository_FindByAccountID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Role
	ctx := context.Background()

	t.Run("finds roles assigned to account", func(t *testing.T) {
		// Create account and role
		account := testpkg.CreateTestAccount(t, db, "roletest")
		role := testpkg.CreateTestRole(t, db, "AccountRole")
		defer cleanupRoleRecords(t, db, role.ID)
		defer cleanupAccountRecords(t, db, account.ID)

		// Assign role to account using direct DB insert (repo method deprecated)
		_, err := db.ExecContext(ctx,
			"INSERT INTO auth.account_roles (account_id, role_id) VALUES (?, ?)",
			account.ID, role.ID)
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
		defer cleanupAccountRecords(t, db, account.ID)

		roles, err := repo.FindByAccountID(ctx, account.ID)
		require.NoError(t, err)
		assert.Empty(t, roles)
	})
}

// ============================================================================
// Role Assignment Tests
// ============================================================================

func TestRoleRepository_GetRoleWithPermissions(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Role
	ctx := context.Background()

	t.Run("gets role with empty permissions", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, db, "WithPerms")
		defer cleanupRoleRecords(t, db, role.ID)

		found, err := repo.GetRoleWithPermissions(ctx, role.ID)
		require.NoError(t, err)
		assert.Equal(t, role.ID, found.ID)
		// Permissions may be empty or nil for a new role
	})
}
