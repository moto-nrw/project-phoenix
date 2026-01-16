package auth_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Note: cleanupAccountRecords and cleanupRoleRecords are defined in role_repository_test.go

// ============================================================================
// AccountRoleRepository CRUD Tests
// ============================================================================

func TestAccountRoleRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).AccountRole
	ctx := context.Background()

	t.Run("creates account-role mapping", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "role_create")
		role := testpkg.CreateTestRole(t, db, "CreateRole")
		defer cleanupAccountRecords(t, db, account.ID)
		defer cleanupRoleRecords(t, db, role.ID)

		ar := &auth.AccountRole{
			AccountID: account.ID,
			RoleID:    role.ID,
		}

		err := repo.Create(ctx, ar)
		require.NoError(t, err)
		assert.NotZero(t, ar.ID)
	})

	t.Run("rejects nil account role", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestAccountRoleRepository_FindByAccountID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).AccountRole
	ctx := context.Background()

	t.Run("finds roles by account ID", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "find_by_acc_role")
		role1 := testpkg.CreateTestRole(t, db, "Role1")
		role2 := testpkg.CreateTestRole(t, db, "Role2")
		defer cleanupAccountRecords(t, db, account.ID)
		defer cleanupRoleRecords(t, db, role1.ID)
		defer cleanupRoleRecords(t, db, role2.ID)

		// Assign both roles
		ar1 := &auth.AccountRole{AccountID: account.ID, RoleID: role1.ID}
		ar2 := &auth.AccountRole{AccountID: account.ID, RoleID: role2.ID}
		require.NoError(t, repo.Create(ctx, ar1))
		require.NoError(t, repo.Create(ctx, ar2))

		// Find by account ID
		roles, err := repo.FindByAccountID(ctx, account.ID)
		require.NoError(t, err)
		assert.Len(t, roles, 2)

		// Verify Role relation is loaded
		for _, ar := range roles {
			assert.NotNil(t, ar.Role)
		}
	})

	t.Run("returns empty slice for account with no roles", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "no_roles")
		defer cleanupAccountRecords(t, db, account.ID)

		roles, err := repo.FindByAccountID(ctx, account.ID)
		require.NoError(t, err)
		assert.Empty(t, roles)
	})
}

func TestAccountRoleRepository_FindByRoleID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).AccountRole
	ctx := context.Background()

	t.Run("finds accounts by role ID", func(t *testing.T) {
		account1 := testpkg.CreateTestAccount(t, db, "find_by_role1")
		account2 := testpkg.CreateTestAccount(t, db, "find_by_role2")
		role := testpkg.CreateTestRole(t, db, "SharedRole")
		defer cleanupAccountRecords(t, db, account1.ID)
		defer cleanupAccountRecords(t, db, account2.ID)
		defer cleanupRoleRecords(t, db, role.ID)

		// Assign role to both accounts
		ar1 := &auth.AccountRole{AccountID: account1.ID, RoleID: role.ID}
		ar2 := &auth.AccountRole{AccountID: account2.ID, RoleID: role.ID}
		require.NoError(t, repo.Create(ctx, ar1))
		require.NoError(t, repo.Create(ctx, ar2))

		// Find by role ID
		accountRoles, err := repo.FindByRoleID(ctx, role.ID)
		require.NoError(t, err)
		assert.Len(t, accountRoles, 2)
	})

	t.Run("returns empty slice for role with no accounts", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, db, "NoAccRole")
		defer cleanupRoleRecords(t, db, role.ID)

		accountRoles, err := repo.FindByRoleID(ctx, role.ID)
		require.NoError(t, err)
		assert.Empty(t, accountRoles)
	})
}

func TestAccountRoleRepository_FindByAccountAndRole(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).AccountRole
	ctx := context.Background()

	t.Run("finds specific account-role mapping", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "find_specific_role")
		role := testpkg.CreateTestRole(t, db, "SpecificRole")
		defer cleanupAccountRecords(t, db, account.ID)
		defer cleanupRoleRecords(t, db, role.ID)

		// Create mapping
		ar := &auth.AccountRole{AccountID: account.ID, RoleID: role.ID}
		require.NoError(t, repo.Create(ctx, ar))

		// Find the specific mapping
		found, err := repo.FindByAccountAndRole(ctx, account.ID, role.ID)
		require.NoError(t, err)
		assert.Equal(t, account.ID, found.AccountID)
		assert.Equal(t, role.ID, found.RoleID)
	})

	t.Run("returns error for non-existent mapping", func(t *testing.T) {
		_, err := repo.FindByAccountAndRole(ctx, 999999, 999999)
		require.Error(t, err)
	})
}

// ============================================================================
// AccountRoleRepository Update Tests
// ============================================================================

func TestAccountRoleRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).AccountRole
	ctx := context.Background()

	t.Run("updates account role mapping", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "update_role")
		role1 := testpkg.CreateTestRole(t, db, "UpdateRole1")
		role2 := testpkg.CreateTestRole(t, db, "UpdateRole2")
		defer cleanupAccountRecords(t, db, account.ID)
		defer cleanupRoleRecords(t, db, role1.ID)
		defer cleanupRoleRecords(t, db, role2.ID)

		// Create initial mapping
		ar := &auth.AccountRole{AccountID: account.ID, RoleID: role1.ID}
		require.NoError(t, repo.Create(ctx, ar))

		// Update to new role
		ar.RoleID = role2.ID
		err := repo.Update(ctx, ar)
		require.NoError(t, err)

		// Verify update - find by new role
		found, err := repo.FindByAccountAndRole(ctx, account.ID, role2.ID)
		require.NoError(t, err)
		assert.Equal(t, role2.ID, found.RoleID)
	})

	t.Run("rejects nil account role", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

// ============================================================================
// AccountRoleRepository Delete Tests
// ============================================================================

func TestAccountRoleRepository_DeleteByAccountAndRole(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).AccountRole
	ctx := context.Background()

	t.Run("deletes existing account-role mapping", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "delete_ar")
		role := testpkg.CreateTestRole(t, db, "DeleteRole")
		defer cleanupAccountRecords(t, db, account.ID)
		defer cleanupRoleRecords(t, db, role.ID)

		// Create mapping
		ar := &auth.AccountRole{AccountID: account.ID, RoleID: role.ID}
		require.NoError(t, repo.Create(ctx, ar))

		// Delete the mapping
		err := repo.DeleteByAccountAndRole(ctx, account.ID, role.ID)
		require.NoError(t, err)

		// Verify deleted
		_, err = repo.FindByAccountAndRole(ctx, account.ID, role.ID)
		require.Error(t, err)
	})

	t.Run("does not error when deleting non-existent mapping", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "delete_none")
		defer cleanupAccountRecords(t, db, account.ID)

		err := repo.DeleteByAccountAndRole(ctx, account.ID, 999999)
		require.NoError(t, err)
	})
}

func TestAccountRoleRepository_DeleteByAccountID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).AccountRole
	ctx := context.Background()

	t.Run("deletes all roles for account", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "delete_all_roles")
		role1 := testpkg.CreateTestRole(t, db, "DeleteAll1")
		role2 := testpkg.CreateTestRole(t, db, "DeleteAll2")
		defer cleanupAccountRecords(t, db, account.ID)
		defer cleanupRoleRecords(t, db, role1.ID)
		defer cleanupRoleRecords(t, db, role2.ID)

		// Create multiple mappings
		ar1 := &auth.AccountRole{AccountID: account.ID, RoleID: role1.ID}
		ar2 := &auth.AccountRole{AccountID: account.ID, RoleID: role2.ID}
		require.NoError(t, repo.Create(ctx, ar1))
		require.NoError(t, repo.Create(ctx, ar2))

		// Delete all roles for account
		err := repo.DeleteByAccountID(ctx, account.ID)
		require.NoError(t, err)

		// Verify all deleted
		roles, err := repo.FindByAccountID(ctx, account.ID)
		require.NoError(t, err)
		assert.Empty(t, roles)
	})

	t.Run("does not error when account has no roles", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "delete_empty")
		defer cleanupAccountRecords(t, db, account.ID)

		err := repo.DeleteByAccountID(ctx, account.ID)
		require.NoError(t, err)
	})
}

func TestAccountRoleRepository_DeleteByRoleID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).AccountRole
	ctx := context.Background()

	t.Run("deletes all account-role mappings for a role", func(t *testing.T) {
		account1 := testpkg.CreateTestAccount(t, db, "del_by_role1")
		account2 := testpkg.CreateTestAccount(t, db, "del_by_role2")
		role := testpkg.CreateTestRole(t, db, "DeleteByRoleID")
		defer cleanupAccountRecords(t, db, account1.ID)
		defer cleanupAccountRecords(t, db, account2.ID)
		defer cleanupRoleRecords(t, db, role.ID)

		// Assign role to both accounts
		ar1 := &auth.AccountRole{AccountID: account1.ID, RoleID: role.ID}
		ar2 := &auth.AccountRole{AccountID: account2.ID, RoleID: role.ID}
		require.NoError(t, repo.Create(ctx, ar1))
		require.NoError(t, repo.Create(ctx, ar2))

		// Verify both mappings exist
		mappings, err := repo.FindByRoleID(ctx, role.ID)
		require.NoError(t, err)
		assert.Len(t, mappings, 2)

		// Delete by role ID
		err = repo.DeleteByRoleID(ctx, role.ID)
		require.NoError(t, err)

		// Verify all mappings are deleted
		mappings, err = repo.FindByRoleID(ctx, role.ID)
		require.NoError(t, err)
		assert.Empty(t, mappings)
	})

	t.Run("does not error when deleting non-existent role mappings", func(t *testing.T) {
		err := repo.DeleteByRoleID(ctx, 999999)
		require.NoError(t, err)
	})
}

// ============================================================================
// AccountRoleRepository List Tests
// ============================================================================

func TestAccountRoleRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).AccountRole
	ctx := context.Background()

	t.Run("lists all account-role mappings", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "list_ar")
		role := testpkg.CreateTestRole(t, db, "ListRole")
		defer cleanupAccountRecords(t, db, account.ID)
		defer cleanupRoleRecords(t, db, role.ID)

		ar := &auth.AccountRole{AccountID: account.ID, RoleID: role.ID}
		require.NoError(t, repo.Create(ctx, ar))

		mappings, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, mappings)
	})

	t.Run("filters by account_id", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "list_by_acc")
		role := testpkg.CreateTestRole(t, db, "ListByAcc")
		defer cleanupAccountRecords(t, db, account.ID)
		defer cleanupRoleRecords(t, db, role.ID)

		ar := &auth.AccountRole{AccountID: account.ID, RoleID: role.ID}
		require.NoError(t, repo.Create(ctx, ar))

		mappings, err := repo.List(ctx, map[string]any{
			"account_id": account.ID,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, mappings)

		for _, m := range mappings {
			assert.Equal(t, account.ID, m.AccountID)
		}
	})

	t.Run("filters by role_id", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "list_by_role")
		role := testpkg.CreateTestRole(t, db, "ListByRole")
		defer cleanupAccountRecords(t, db, account.ID)
		defer cleanupRoleRecords(t, db, role.ID)

		ar := &auth.AccountRole{AccountID: account.ID, RoleID: role.ID}
		require.NoError(t, repo.Create(ctx, ar))

		mappings, err := repo.List(ctx, map[string]any{
			"role_id": role.ID,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, mappings)

		for _, m := range mappings {
			assert.Equal(t, role.ID, m.RoleID)
		}
	})
}

func TestAccountRoleRepository_FindAccountRolesWithDetails(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).AccountRole
	ctx := context.Background()

	t.Run("finds roles with account and role details", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "with_details")
		role := testpkg.CreateTestRole(t, db, "WithDetailsRole")
		defer cleanupAccountRecords(t, db, account.ID)
		defer cleanupRoleRecords(t, db, role.ID)

		ar := &auth.AccountRole{AccountID: account.ID, RoleID: role.ID}
		require.NoError(t, repo.Create(ctx, ar))

		mappings, err := repo.FindAccountRolesWithDetails(ctx, map[string]any{
			"account_id": account.ID,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, mappings)
	})

	t.Run("filters by role_id with details", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "details_role")
		role := testpkg.CreateTestRole(t, db, "DetailsRole")
		defer cleanupAccountRecords(t, db, account.ID)
		defer cleanupRoleRecords(t, db, role.ID)

		ar := &auth.AccountRole{AccountID: account.ID, RoleID: role.ID}
		require.NoError(t, repo.Create(ctx, ar))

		mappings, err := repo.FindAccountRolesWithDetails(ctx, map[string]any{
			"role_id": role.ID,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, mappings)
	})
}
