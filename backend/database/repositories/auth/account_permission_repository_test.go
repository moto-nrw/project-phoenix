package auth_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func findAccountPermission(
	t *testing.T,
	repo auth.AccountPermissionRepository,
	ctx context.Context,
	accountID, permissionID int64,
) *auth.AccountPermission {
	t.Helper()
	permissions, err := repo.FindByAccountID(ctx, accountID)
	require.NoError(t, err)
	for _, permission := range permissions {
		if permission.PermissionID == permissionID {
			return permission
		}
	}
	return nil
}

// ============================================================================
// AccountPermissionRepository CRUD Tests
// ============================================================================

func TestAccountPermissionRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).AccountPermission
	ctx := testpkg.Ctx(t)

	t.Run("creates account permission mapping", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "perm_create")
		permission := testpkg.CreateTestPermission(t, db, "TestPerm", "test_resource", "read")

		ap := &auth.AccountPermission{
			AccountID:    account.ID,
			PermissionID: permission.ID,
			Granted:      true,
		}

		err := repo.Create(ctx, ap)
		require.NoError(t, err)
		assert.NotZero(t, ap.ID)

	})

	t.Run("rejects nil account permission", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestAccountPermissionRepository_FindByAccountID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).AccountPermission
	ctx := testpkg.Ctx(t)

	t.Run("finds permissions by account ID", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "find_by_acc")
		permission1 := testpkg.CreateTestPermission(t, db, "Perm1", "resource1", "read")
		permission2 := testpkg.CreateTestPermission(t, db, "Perm2", "resource2", "write")

		// Grant both permissions
		err := repo.GrantPermission(ctx, account.ID, permission1.ID)
		require.NoError(t, err)
		err = repo.GrantPermission(ctx, account.ID, permission2.ID)
		require.NoError(t, err)

		// Find by account ID
		perms, err := repo.FindByAccountID(ctx, account.ID)
		require.NoError(t, err)
		assert.Len(t, perms, 2)
	})

	t.Run("returns empty slice for account with no permissions", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "no_perms")

		perms, err := repo.FindByAccountID(ctx, account.ID)
		require.NoError(t, err)
		assert.Empty(t, perms)
	})
}

// ============================================================================
// AccountPermissionRepository Grant/Deny/Remove Tests
// ============================================================================

func TestAccountPermissionRepository_GrantPermission(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).AccountPermission
	ctx := testpkg.Ctx(t)

	t.Run("grants new permission", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "grant_new")
		permission := testpkg.CreateTestPermission(t, db, "GrantPerm", "grant", "read")

		err := repo.GrantPermission(ctx, account.ID, permission.ID)
		require.NoError(t, err)

		// Verify granted
		mapping := findAccountPermission(t, repo, ctx, account.ID, permission.ID)
		require.NotNil(t, mapping)
		assert.True(t, mapping.Granted)
	})

	t.Run("updates existing permission to granted", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "grant_update")
		permission := testpkg.CreateTestPermission(t, db, "UpdatePerm", "update", "write")

		// First deny the permission
		err := repo.DenyPermission(ctx, account.ID, permission.ID)
		require.NoError(t, err)

		// Now grant it
		err = repo.GrantPermission(ctx, account.ID, permission.ID)
		require.NoError(t, err)

		// Verify granted
		mapping := findAccountPermission(t, repo, ctx, account.ID, permission.ID)
		require.NotNil(t, mapping)
		assert.True(t, mapping.Granted)
	})
}

func TestAccountPermissionRepository_DenyPermission(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).AccountPermission
	ctx := testpkg.Ctx(t)

	t.Run("denies new permission", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "deny_new")
		permission := testpkg.CreateTestPermission(t, db, "DenyPerm", "deny", "delete")

		err := repo.DenyPermission(ctx, account.ID, permission.ID)
		require.NoError(t, err)

		// Verify denied
		mapping := findAccountPermission(t, repo, ctx, account.ID, permission.ID)
		require.NotNil(t, mapping)
		assert.False(t, mapping.Granted)
	})

	t.Run("updates existing permission to denied", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "deny_update")
		permission := testpkg.CreateTestPermission(t, db, "DenyUpdate", "deny_update", "read")

		// First grant the permission
		err := repo.GrantPermission(ctx, account.ID, permission.ID)
		require.NoError(t, err)

		// Now deny it
		err = repo.DenyPermission(ctx, account.ID, permission.ID)
		require.NoError(t, err)

		// Verify denied
		mapping := findAccountPermission(t, repo, ctx, account.ID, permission.ID)
		require.NotNil(t, mapping)
		assert.False(t, mapping.Granted)
	})
}

func TestAccountPermissionRepository_RemovePermission(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).AccountPermission
	ctx := testpkg.Ctx(t)

	t.Run("removes existing permission", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "remove_perm")
		permission := testpkg.CreateTestPermission(t, db, "RemovePerm", "remove", "read")

		// First grant the permission
		err := repo.GrantPermission(ctx, account.ID, permission.ID)
		require.NoError(t, err)

		// Now remove it
		err = repo.RemovePermission(ctx, account.ID, permission.ID)
		require.NoError(t, err)

		// Verify removed
		assert.Nil(t, findAccountPermission(t, repo, ctx, account.ID, permission.ID))
	})

	t.Run("does not error when removing non-existent permission", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "remove_none")

		// Remove a permission that was never granted
		err := repo.RemovePermission(ctx, account.ID, 999999)
		require.NoError(t, err) // Should not error
	})
}

// ============================================================================
// AccountPermissionRepository Update Tests
// ============================================================================

func TestAccountPermissionRepository_Update(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).AccountPermission
	ctx := testpkg.Ctx(t)

	t.Run("updates account permission granted status", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "update_ap")
		permission := testpkg.CreateTestPermission(t, db, "UpdateAP", "updateap", "read")

		// Create initial permission
		ap := &auth.AccountPermission{
			AccountID:    account.ID,
			PermissionID: permission.ID,
			Granted:      true,
		}
		err := repo.Create(ctx, ap)
		require.NoError(t, err)

		// Update to denied
		ap.Granted = false
		err = repo.Update(ctx, ap)
		require.NoError(t, err)

		// Verify update
		mapping := findAccountPermission(t, repo, ctx, account.ID, permission.ID)
		require.NotNil(t, mapping)
		assert.False(t, mapping.Granted)
	})

	t.Run("rejects nil account permission", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

// ============================================================================
// AccountPermissionRepository List Tests
// ============================================================================

func TestAccountPermissionRepository_List(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).AccountPermission
	ctx := testpkg.Ctx(t)

	t.Run("lists all account permissions", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "list_ap")
		permission := testpkg.CreateTestPermission(t, db, "ListPerm", "listperm", "read")

		err := repo.GrantPermission(ctx, account.ID, permission.ID)
		require.NoError(t, err)

		perms, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, perms)
	})

	t.Run("filters by granted status", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "list_granted")
		permission := testpkg.CreateTestPermission(t, db, "GrantedPerm", "granted", "read")

		err := repo.GrantPermission(ctx, account.ID, permission.ID)
		require.NoError(t, err)

		perms, err := repo.List(ctx, map[string]any{
			"granted": true,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, perms)

		for _, p := range perms {
			assert.True(t, p.Granted)
		}
	})

	t.Run("filters by account_id", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "list_by_acc")
		permission := testpkg.CreateTestPermission(t, db, "ByAccPerm", "byacc", "read")

		err := repo.GrantPermission(ctx, account.ID, permission.ID)
		require.NoError(t, err)

		perms, err := repo.List(ctx, map[string]any{
			"account_id": account.ID,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, perms)

		for _, p := range perms {
			assert.Equal(t, account.ID, p.AccountID)
		}
	})
}

// ============================================================================
// AccountPermissionRepository DeleteByPermissionID Tests
// ============================================================================

func TestAccountPermissionRepository_DeleteByPermissionID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	repo := repos.AccountPermission
	ctx := testpkg.Ctx(t)

	t.Run("deletes all account permissions for a permission", func(t *testing.T) {
		account1 := testpkg.CreateTestAccount(t, db, "del_by_perm1")
		account2 := testpkg.CreateTestAccount(t, db, "del_by_perm2")
		permission := testpkg.CreateTestPermission(t, db, "DeletePerm", "delete", "read")

		// Grant permission to both accounts
		err := repo.GrantPermission(ctx, account1.ID, permission.ID)
		require.NoError(t, err)
		err = repo.GrantPermission(ctx, account2.ID, permission.ID)
		require.NoError(t, err)

		// Verify both exist
		assert.NotNil(t, findAccountPermission(t, repo, ctx, account1.ID, permission.ID))
		assert.NotNil(t, findAccountPermission(t, repo, ctx, account2.ID, permission.ID))

		// Delete by permission ID
		err = repo.DeleteByPermissionID(ctx, permission.ID)
		require.NoError(t, err)

		// Verify all deleted
		assert.Nil(t, findAccountPermission(t, repo, ctx, account1.ID, permission.ID))
		assert.Nil(t, findAccountPermission(t, repo, ctx, account2.ID, permission.ID))
	})

	t.Run("does not error when deleting non-existent permission mappings", func(t *testing.T) {
		err := repo.DeleteByPermissionID(ctx, 999999)
		require.NoError(t, err)
	})
}
