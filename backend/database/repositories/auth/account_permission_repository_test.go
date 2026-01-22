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

// Note: cleanupAccountRecords is defined in role_repository_test.go

// cleanupAccountPermission removes an account-permission mapping from the database.
func cleanupAccountPermission(t *testing.T, db *bun.DB, accountID, permissionID int64) {
	t.Helper()
	_, _ = db.NewDelete().
		Model((*interface{})(nil)).
		Table("auth.account_permissions").
		Where("account_id = ? AND permission_id = ?", accountID, permissionID).
		Exec(context.Background())
}

// cleanupPermissionByID removes a permission by ID.
func cleanupPermissionByID(t *testing.T, db *bun.DB, permissionID int64) {
	t.Helper()
	ctx := context.Background()

	// First clean up any account_permissions referencing this permission
	_, _ = db.NewDelete().
		Model((*interface{})(nil)).
		Table("auth.account_permissions").
		Where("permission_id = ?", permissionID).
		Exec(ctx)

	// Clean up any role_permissions referencing this permission
	_, _ = db.NewDelete().
		Model((*any)(nil)).
		Table("auth.role_permissions").
		Where("permission_id = ?", permissionID).
		Exec(ctx)

	// Then delete the permission itself
	_, _ = db.NewDelete().
		Model((*interface{})(nil)).
		Table("auth.permissions").
		Where("id = ?", permissionID).
		Exec(ctx)
}

// ============================================================================
// AccountPermissionRepository CRUD Tests
// ============================================================================

func TestAccountPermissionRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).AccountPermission
	ctx := context.Background()

	t.Run("creates account permission mapping", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "perm_create")
		permission := testpkg.CreateTestPermission(t, db, "TestPerm", "test_resource", "read")
		defer cleanupAccountRecords(t, db, account.ID)
		defer cleanupPermissionByID(t, db, permission.ID)

		ap := &auth.AccountPermission{
			AccountID:    account.ID,
			PermissionID: permission.ID,
			Granted:      true,
		}

		err := repo.Create(ctx, ap)
		require.NoError(t, err)
		assert.NotZero(t, ap.ID)

		defer cleanupAccountPermission(t, db, account.ID, permission.ID)
	})

	t.Run("rejects nil account permission", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestAccountPermissionRepository_FindByAccountID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).AccountPermission
	ctx := context.Background()

	t.Run("finds permissions by account ID", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "find_by_acc")
		permission1 := testpkg.CreateTestPermission(t, db, "Perm1", "resource1", "read")
		permission2 := testpkg.CreateTestPermission(t, db, "Perm2", "resource2", "write")
		defer cleanupAccountRecords(t, db, account.ID)
		defer cleanupPermissionByID(t, db, permission1.ID)
		defer cleanupPermissionByID(t, db, permission2.ID)

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
		defer cleanupAccountRecords(t, db, account.ID)

		perms, err := repo.FindByAccountID(ctx, account.ID)
		require.NoError(t, err)
		assert.Empty(t, perms)
	})
}

func TestAccountPermissionRepository_FindByPermissionID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).AccountPermission
	ctx := context.Background()

	t.Run("finds accounts by permission ID", func(t *testing.T) {
		account1 := testpkg.CreateTestAccount(t, db, "find_by_perm1")
		account2 := testpkg.CreateTestAccount(t, db, "find_by_perm2")
		permission := testpkg.CreateTestPermission(t, db, "SharedPerm", "shared", "read")
		defer cleanupAccountRecords(t, db, account1.ID)
		defer cleanupAccountRecords(t, db, account2.ID)
		defer cleanupPermissionByID(t, db, permission.ID)

		// Grant permission to both accounts
		err := repo.GrantPermission(ctx, account1.ID, permission.ID)
		require.NoError(t, err)
		err = repo.GrantPermission(ctx, account2.ID, permission.ID)
		require.NoError(t, err)

		// Find by permission ID
		accountPerms, err := repo.FindByPermissionID(ctx, permission.ID)
		require.NoError(t, err)
		assert.Len(t, accountPerms, 2)
	})
}

func TestAccountPermissionRepository_FindByAccountAndPermission(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).AccountPermission
	ctx := context.Background()

	t.Run("finds specific account-permission mapping", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "find_specific")
		permission := testpkg.CreateTestPermission(t, db, "SpecificPerm", "specific", "read")
		defer cleanupAccountRecords(t, db, account.ID)
		defer cleanupPermissionByID(t, db, permission.ID)

		// Grant permission
		err := repo.GrantPermission(ctx, account.ID, permission.ID)
		require.NoError(t, err)

		// Find the specific mapping
		ap, err := repo.FindByAccountAndPermission(ctx, account.ID, permission.ID)
		require.NoError(t, err)
		assert.Equal(t, account.ID, ap.AccountID)
		assert.Equal(t, permission.ID, ap.PermissionID)
		assert.True(t, ap.Granted)
	})

	t.Run("returns error for non-existent mapping", func(t *testing.T) {
		_, err := repo.FindByAccountAndPermission(ctx, 999999, 999999)
		require.Error(t, err)
	})
}

// ============================================================================
// AccountPermissionRepository Grant/Deny/Remove Tests
// ============================================================================

func TestAccountPermissionRepository_GrantPermission(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).AccountPermission
	ctx := context.Background()

	t.Run("grants new permission", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "grant_new")
		permission := testpkg.CreateTestPermission(t, db, "GrantPerm", "grant", "read")
		defer cleanupAccountRecords(t, db, account.ID)
		defer cleanupPermissionByID(t, db, permission.ID)

		err := repo.GrantPermission(ctx, account.ID, permission.ID)
		require.NoError(t, err)

		// Verify granted
		ap, err := repo.FindByAccountAndPermission(ctx, account.ID, permission.ID)
		require.NoError(t, err)
		assert.True(t, ap.Granted)
	})

	t.Run("updates existing permission to granted", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "grant_update")
		permission := testpkg.CreateTestPermission(t, db, "UpdatePerm", "update", "write")
		defer cleanupAccountRecords(t, db, account.ID)
		defer cleanupPermissionByID(t, db, permission.ID)

		// First deny the permission
		err := repo.DenyPermission(ctx, account.ID, permission.ID)
		require.NoError(t, err)

		// Now grant it
		err = repo.GrantPermission(ctx, account.ID, permission.ID)
		require.NoError(t, err)

		// Verify granted
		ap, err := repo.FindByAccountAndPermission(ctx, account.ID, permission.ID)
		require.NoError(t, err)
		assert.True(t, ap.Granted)
	})
}

func TestAccountPermissionRepository_DenyPermission(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).AccountPermission
	ctx := context.Background()

	t.Run("denies new permission", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "deny_new")
		permission := testpkg.CreateTestPermission(t, db, "DenyPerm", "deny", "delete")
		defer cleanupAccountRecords(t, db, account.ID)
		defer cleanupPermissionByID(t, db, permission.ID)

		err := repo.DenyPermission(ctx, account.ID, permission.ID)
		require.NoError(t, err)

		// Verify denied
		ap, err := repo.FindByAccountAndPermission(ctx, account.ID, permission.ID)
		require.NoError(t, err)
		assert.False(t, ap.Granted)
	})

	t.Run("updates existing permission to denied", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "deny_update")
		permission := testpkg.CreateTestPermission(t, db, "DenyUpdate", "deny_update", "read")
		defer cleanupAccountRecords(t, db, account.ID)
		defer cleanupPermissionByID(t, db, permission.ID)

		// First grant the permission
		err := repo.GrantPermission(ctx, account.ID, permission.ID)
		require.NoError(t, err)

		// Now deny it
		err = repo.DenyPermission(ctx, account.ID, permission.ID)
		require.NoError(t, err)

		// Verify denied
		ap, err := repo.FindByAccountAndPermission(ctx, account.ID, permission.ID)
		require.NoError(t, err)
		assert.False(t, ap.Granted)
	})
}

func TestAccountPermissionRepository_RemovePermission(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).AccountPermission
	ctx := context.Background()

	t.Run("removes existing permission", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "remove_perm")
		permission := testpkg.CreateTestPermission(t, db, "RemovePerm", "remove", "read")
		defer cleanupAccountRecords(t, db, account.ID)
		defer cleanupPermissionByID(t, db, permission.ID)

		// First grant the permission
		err := repo.GrantPermission(ctx, account.ID, permission.ID)
		require.NoError(t, err)

		// Now remove it
		err = repo.RemovePermission(ctx, account.ID, permission.ID)
		require.NoError(t, err)

		// Verify removed
		_, err = repo.FindByAccountAndPermission(ctx, account.ID, permission.ID)
		require.Error(t, err) // Should not find it
	})

	t.Run("does not error when removing non-existent permission", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "remove_none")
		defer cleanupAccountRecords(t, db, account.ID)

		// Remove a permission that was never granted
		err := repo.RemovePermission(ctx, account.ID, 999999)
		require.NoError(t, err) // Should not error
	})
}

// ============================================================================
// AccountPermissionRepository Update Tests
// ============================================================================

func TestAccountPermissionRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).AccountPermission
	ctx := context.Background()

	t.Run("updates account permission granted status", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "update_ap")
		permission := testpkg.CreateTestPermission(t, db, "UpdateAP", "updateap", "read")
		defer cleanupAccountRecords(t, db, account.ID)
		defer cleanupPermissionByID(t, db, permission.ID)

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
		found, err := repo.FindByAccountAndPermission(ctx, account.ID, permission.ID)
		require.NoError(t, err)
		assert.False(t, found.Granted)
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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).AccountPermission
	ctx := context.Background()

	t.Run("lists all account permissions", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "list_ap")
		permission := testpkg.CreateTestPermission(t, db, "ListPerm", "listperm", "read")
		defer cleanupAccountRecords(t, db, account.ID)
		defer cleanupPermissionByID(t, db, permission.ID)

		err := repo.GrantPermission(ctx, account.ID, permission.ID)
		require.NoError(t, err)

		perms, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, perms)
	})

	t.Run("filters by granted status", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "list_granted")
		permission := testpkg.CreateTestPermission(t, db, "GrantedPerm", "granted", "read")
		defer cleanupAccountRecords(t, db, account.ID)
		defer cleanupPermissionByID(t, db, permission.ID)

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
		defer cleanupAccountRecords(t, db, account.ID)
		defer cleanupPermissionByID(t, db, permission.ID)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).AccountPermission
	ctx := context.Background()

	t.Run("deletes all account permissions for a permission", func(t *testing.T) {
		account1 := testpkg.CreateTestAccount(t, db, "del_by_perm1")
		account2 := testpkg.CreateTestAccount(t, db, "del_by_perm2")
		permission := testpkg.CreateTestPermission(t, db, "DeletePerm", "delete", "read")
		defer cleanupAccountRecords(t, db, account1.ID)
		defer cleanupAccountRecords(t, db, account2.ID)
		defer cleanupPermissionByID(t, db, permission.ID)

		// Grant permission to both accounts
		err := repo.GrantPermission(ctx, account1.ID, permission.ID)
		require.NoError(t, err)
		err = repo.GrantPermission(ctx, account2.ID, permission.ID)
		require.NoError(t, err)

		// Verify both exist
		perms, err := repo.FindByPermissionID(ctx, permission.ID)
		require.NoError(t, err)
		assert.Len(t, perms, 2)

		// Delete by permission ID
		err = repo.DeleteByPermissionID(ctx, permission.ID)
		require.NoError(t, err)

		// Verify all deleted
		perms, err = repo.FindByPermissionID(ctx, permission.ID)
		require.NoError(t, err)
		assert.Empty(t, perms)
	})

	t.Run("does not error when deleting non-existent permission mappings", func(t *testing.T) {
		err := repo.DeleteByPermissionID(ctx, 999999)
		require.NoError(t, err)
	})
}

func TestAccountPermissionRepository_FindAccountPermissionsWithDetails(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	repo := repositories.NewFactory(db).AccountPermission
	ctx := context.Background()

	t.Run("finds permissions with account and permission details", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "with_details")
		permission := testpkg.CreateTestPermission(t, db, "DetailsPerm", "details", "read")
		defer cleanupAccountRecords(t, db, account.ID)
		defer cleanupPermissionByID(t, db, permission.ID)

		err := repo.GrantPermission(ctx, account.ID, permission.ID)
		require.NoError(t, err)

		perms, err := repo.FindAccountPermissionsWithDetails(ctx, map[string]any{
			"account_id": account.ID,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, perms)
	})

	t.Run("filters by permission_id", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "details_perm_id")
		permission := testpkg.CreateTestPermission(t, db, "DetailsPermID", "detailspermid", "write")
		defer cleanupAccountRecords(t, db, account.ID)
		defer cleanupPermissionByID(t, db, permission.ID)

		err := repo.GrantPermission(ctx, account.ID, permission.ID)
		require.NoError(t, err)

		perms, err := repo.FindAccountPermissionsWithDetails(ctx, map[string]any{
			"permission_id": permission.ID,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, perms)
	})
}
