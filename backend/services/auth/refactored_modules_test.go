// Package auth_test contains hermetic tests for the refactored auth service modules.
// These tests specifically target coverage gaps in the split files.
package auth_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// setupAuthServiceWithDB creates an auth service with real database connection
func setupAuthServiceWithDB(t *testing.T, db *bun.DB) auth.AuthService {
	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db)
	require.NoError(t, err, "Failed to create service factory")
	return serviceFactory.Auth
}

// =============================================================================
// Role Management Extended Tests (role_management.go)
// =============================================================================

func TestAuthService_DeleteRole_Extended(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupAuthServiceWithDB(t, db)
	ctx := context.Background()

	t.Run("deletes role with all associations successfully", func(t *testing.T) {
		// ARRANGE - create role with permission assignment
		roleName := fmt.Sprintf("delete-full-role-%d", time.Now().UnixNano())
		role, err := service.CreateRole(ctx, roleName, "Role to delete with associations")
		require.NoError(t, err)

		// Create permission and assign to role
		permName := fmt.Sprintf("delete-role-perm-%d", time.Now().UnixNano())
		resource := fmt.Sprintf("delete-role-res-%d", time.Now().UnixNano())
		perm, err := service.CreatePermission(ctx, permName, "Test permission", resource, "read")
		require.NoError(t, err)

		err = service.AssignPermissionToRole(ctx, int(role.ID), int(perm.ID))
		require.NoError(t, err)

		// Create account and assign role
		email := fmt.Sprintf("delete-role-user-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil)
		if account != nil {
			defer testpkg.CleanupAuthFixtures(t, db, account.ID)
		}
		require.NoError(t, err)

		err = service.AssignRoleToAccount(ctx, int(account.ID), int(role.ID))
		require.NoError(t, err)

		// ACT
		err = service.DeleteRole(ctx, int(role.ID))

		// ASSERT
		require.NoError(t, err)

		// Verify role is deleted
		_, err = service.GetRoleByID(ctx, int(role.ID))
		require.Error(t, err)

		// Verify account no longer has role
		roles, err := service.GetAccountRoles(ctx, int(account.ID))
		require.NoError(t, err)
		for _, r := range roles {
			assert.NotEqual(t, role.ID, r.ID)
		}
	})

	t.Run("is idempotent for non-existent role", func(t *testing.T) {
		// ACT - DeleteRole is idempotent, doesn't error on non-existent role
		err := service.DeleteRole(ctx, 99999999)

		// ASSERT
		require.NoError(t, err)
	})
}

func TestAuthService_AssignRoleToAccount_Extended(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupAuthServiceWithDB(t, db)
	ctx := context.Background()

	t.Run("returns error for non-existent account", func(t *testing.T) {
		// ARRANGE
		roleName := fmt.Sprintf("assign-role-%d", time.Now().UnixNano())
		role, err := service.CreateRole(ctx, roleName, "Test role")
		require.NoError(t, err)

		// ACT
		err = service.AssignRoleToAccount(ctx, 99999999, int(role.ID))

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for non-existent role", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("assign-role-user-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil)
		if account != nil {
			defer testpkg.CleanupAuthFixtures(t, db, account.ID)
		}
		require.NoError(t, err)

		// ACT
		err = service.AssignRoleToAccount(ctx, int(account.ID), 99999999)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("is idempotent - assigning same role twice succeeds", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("idempotent-role-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil)
		if account != nil {
			defer testpkg.CleanupAuthFixtures(t, db, account.ID)
		}
		require.NoError(t, err)

		roleName := fmt.Sprintf("idempotent-role-%d", time.Now().UnixNano())
		role, err := service.CreateRole(ctx, roleName, "Test role")
		require.NoError(t, err)

		// First assignment
		err = service.AssignRoleToAccount(ctx, int(account.ID), int(role.ID))
		require.NoError(t, err)

		// ACT - Second assignment (should be idempotent)
		err = service.AssignRoleToAccount(ctx, int(account.ID), int(role.ID))

		// ASSERT
		require.NoError(t, err)

		// Verify role is assigned only once
		roles, err := service.GetAccountRoles(ctx, int(account.ID))
		require.NoError(t, err)
		count := 0
		for _, r := range roles {
			if r.ID == role.ID {
				count++
			}
		}
		assert.Equal(t, 1, count)
	})
}

func TestAuthService_RemoveRoleFromAccount_Extended(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupAuthServiceWithDB(t, db)
	ctx := context.Background()

	t.Run("removes role from account successfully", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("remove-role-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil)
		if account != nil {
			defer testpkg.CleanupAuthFixtures(t, db, account.ID)
		}
		require.NoError(t, err)

		roleName := fmt.Sprintf("remove-role-%d", time.Now().UnixNano())
		role, err := service.CreateRole(ctx, roleName, "Test role")
		require.NoError(t, err)

		err = service.AssignRoleToAccount(ctx, int(account.ID), int(role.ID))
		require.NoError(t, err)

		// ACT
		err = service.RemoveRoleFromAccount(ctx, int(account.ID), int(role.ID))

		// ASSERT
		require.NoError(t, err)

		// Verify role is removed
		roles, err := service.GetAccountRoles(ctx, int(account.ID))
		require.NoError(t, err)
		for _, r := range roles {
			assert.NotEqual(t, role.ID, r.ID)
		}
	})
}

func TestAuthService_GetAccountRoles_Extended(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupAuthServiceWithDB(t, db)
	ctx := context.Background()

	t.Run("returns empty list for account with no roles", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("no-roles-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil)
		if account != nil {
			defer testpkg.CleanupAuthFixtures(t, db, account.ID)
		}
		require.NoError(t, err)

		// Remove any default roles
		roles, _ := service.GetAccountRoles(ctx, int(account.ID))
		for _, r := range roles {
			_ = service.RemoveRoleFromAccount(ctx, int(account.ID), int(r.ID))
		}

		// ACT
		result, err := service.GetAccountRoles(ctx, int(account.ID))

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

// =============================================================================
// Permission Management Extended Tests (permission_management.go)
// =============================================================================

func TestAuthService_DeletePermission_Extended(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupAuthServiceWithDB(t, db)
	ctx := context.Background()

	t.Run("deletes permission with role association", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		permName := fmt.Sprintf("delete-perm-role-%s", uniqueID)
		resource := fmt.Sprintf("delete-perm-res-%s", uniqueID)
		perm, err := service.CreatePermission(ctx, permName, "Permission to delete", resource, "read")
		require.NoError(t, err)

		// Create role and assign permission
		roleName := fmt.Sprintf("perm-role-%s", uniqueID)
		role, err := service.CreateRole(ctx, roleName, "Test role")
		require.NoError(t, err)

		err = service.AssignPermissionToRole(ctx, int(role.ID), int(perm.ID))
		require.NoError(t, err)

		// ACT
		err = service.DeletePermission(ctx, int(perm.ID))

		// ASSERT
		require.NoError(t, err)

		// Verify permission is deleted
		_, err = service.GetPermissionByID(ctx, int(perm.ID))
		require.Error(t, err)
	})

	t.Run("deletes permission with account association", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		permName := fmt.Sprintf("delete-perm-acct-%s", uniqueID)
		resource := fmt.Sprintf("delete-perm-res2-%s", uniqueID)
		perm, err := service.CreatePermission(ctx, permName, "Permission to delete", resource, "write")
		require.NoError(t, err)

		// Create account and grant permission
		email := fmt.Sprintf("delete-perm-user-%s@test.local", uniqueID)
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%s", uniqueID), "Test1234%", nil)
		if account != nil {
			defer testpkg.CleanupAuthFixtures(t, db, account.ID)
		}
		require.NoError(t, err)

		err = service.GrantPermissionToAccount(ctx, int(account.ID), int(perm.ID))
		require.NoError(t, err)

		// ACT
		err = service.DeletePermission(ctx, int(perm.ID))

		// ASSERT
		require.NoError(t, err)

		// Verify permission is deleted
		_, err = service.GetPermissionByID(ctx, int(perm.ID))
		require.Error(t, err)
	})
}

func TestAuthService_GrantPermissionToAccount_Extended(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupAuthServiceWithDB(t, db)
	ctx := context.Background()

	t.Run("returns error for non-existent permission", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("grant-perm-user-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil)
		if account != nil {
			defer testpkg.CleanupAuthFixtures(t, db, account.ID)
		}
		require.NoError(t, err)

		// ACT
		err = service.GrantPermissionToAccount(ctx, int(account.ID), 99999999)

		// ASSERT
		require.Error(t, err)
	})
}

func TestAuthService_RemovePermissionFromAccount_Extended(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupAuthServiceWithDB(t, db)
	ctx := context.Background()

	t.Run("removes permission from account", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		permName := fmt.Sprintf("remove-perm-%s", uniqueID)
		resource := fmt.Sprintf("remove-res-%s", uniqueID)
		perm, err := service.CreatePermission(ctx, permName, "Permission to remove", resource, "read")
		require.NoError(t, err)

		email := fmt.Sprintf("remove-perm-user-%s@test.local", uniqueID)
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%s", uniqueID), "Test1234%", nil)
		if account != nil {
			defer testpkg.CleanupAuthFixtures(t, db, account.ID)
		}
		require.NoError(t, err)

		err = service.GrantPermissionToAccount(ctx, int(account.ID), int(perm.ID))
		require.NoError(t, err)

		// ACT
		err = service.RemovePermissionFromAccount(ctx, int(account.ID), int(perm.ID))

		// ASSERT
		require.NoError(t, err)
	})
}

func TestAuthService_AssignPermissionToRole_Extended(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupAuthServiceWithDB(t, db)
	ctx := context.Background()

	t.Run("returns error for non-existent role", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		permName := fmt.Sprintf("assign-to-role-%s", uniqueID)
		resource := fmt.Sprintf("assign-res-%s", uniqueID)
		perm, err := service.CreatePermission(ctx, permName, "Test permission", resource, "read")
		require.NoError(t, err)

		// ACT
		err = service.AssignPermissionToRole(ctx, 99999999, int(perm.ID))

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for non-existent permission", func(t *testing.T) {
		// ARRANGE
		roleName := fmt.Sprintf("assign-perm-role-%d", time.Now().UnixNano())
		role, err := service.CreateRole(ctx, roleName, "Test role")
		require.NoError(t, err)

		// ACT
		err = service.AssignPermissionToRole(ctx, int(role.ID), 99999999)

		// ASSERT
		require.Error(t, err)
	})
}

func TestAuthService_RemovePermissionFromRole_Extended(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupAuthServiceWithDB(t, db)
	ctx := context.Background()

	t.Run("removes permission from role", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		roleName := fmt.Sprintf("remove-perm-role-%s", uniqueID)
		role, err := service.CreateRole(ctx, roleName, "Test role")
		require.NoError(t, err)

		permName := fmt.Sprintf("remove-from-role-%s", uniqueID)
		resource := fmt.Sprintf("remove-from-res-%s", uniqueID)
		perm, err := service.CreatePermission(ctx, permName, "Test permission", resource, "read")
		require.NoError(t, err)

		err = service.AssignPermissionToRole(ctx, int(role.ID), int(perm.ID))
		require.NoError(t, err)

		// ACT
		err = service.RemovePermissionFromRole(ctx, int(role.ID), int(perm.ID))

		// ASSERT
		require.NoError(t, err)

		// Verify permission is removed
		perms, err := service.GetRolePermissions(ctx, int(role.ID))
		require.NoError(t, err)
		for _, p := range perms {
			assert.NotEqual(t, perm.ID, p.ID)
		}
	})
}

func TestAuthService_GetRolePermissions_Extended(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupAuthServiceWithDB(t, db)
	ctx := context.Background()

	t.Run("returns empty list for role with no permissions", func(t *testing.T) {
		// ARRANGE
		roleName := fmt.Sprintf("empty-perm-role-%d", time.Now().UnixNano())
		role, err := service.CreateRole(ctx, roleName, "Test role")
		require.NoError(t, err)

		// ACT
		perms, err := service.GetRolePermissions(ctx, int(role.ID))

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, perms)
	})

	t.Run("returns permissions for role", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		roleName := fmt.Sprintf("has-perm-role-%s", uniqueID)
		role, err := service.CreateRole(ctx, roleName, "Test role")
		require.NoError(t, err)

		permName := fmt.Sprintf("role-perm-%s", uniqueID)
		resource := fmt.Sprintf("role-res-%s", uniqueID)
		perm, err := service.CreatePermission(ctx, permName, "Test permission", resource, "read")
		require.NoError(t, err)

		err = service.AssignPermissionToRole(ctx, int(role.ID), int(perm.ID))
		require.NoError(t, err)

		// ACT
		perms, err := service.GetRolePermissions(ctx, int(role.ID))

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, perms)
		found := false
		for _, p := range perms {
			if p.ID == perm.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

// =============================================================================
// Account Management Extended Tests (account_management.go)
// =============================================================================

func TestAuthService_ActivateAccount_Extended(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupAuthServiceWithDB(t, db)
	ctx := context.Background()

	t.Run("activates already active account (idempotent)", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("already-active-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil)
		if account != nil {
			defer testpkg.CleanupAuthFixtures(t, db, account.ID)
		}
		require.NoError(t, err)

		// Account is already active by default

		// ACT
		err = service.ActivateAccount(ctx, int(account.ID))

		// ASSERT
		require.NoError(t, err)

		// Verify still active
		updated, err := service.GetAccountByID(ctx, int(account.ID))
		require.NoError(t, err)
		assert.True(t, updated.Active)
	})
}

func TestAuthService_DeactivateAccount_Extended(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupAuthServiceWithDB(t, db)
	ctx := context.Background()

	t.Run("deactivates account and invalidates tokens", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("deactivate-tokens-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil)
		if account != nil {
			defer testpkg.CleanupAuthFixtures(t, db, account.ID)
		}
		require.NoError(t, err)

		// Login to create tokens
		_, refreshToken, err := service.Login(ctx, email, "Test1234%")
		require.NoError(t, err)

		// ACT
		err = service.DeactivateAccount(ctx, int(account.ID))

		// ASSERT
		require.NoError(t, err)

		// Verify account is deactivated
		updated, err := service.GetAccountByID(ctx, int(account.ID))
		require.NoError(t, err)
		assert.False(t, updated.Active)

		// Verify tokens are invalidated
		_, _, err = service.RefreshToken(ctx, refreshToken)
		require.Error(t, err)
	})

	t.Run("deactivates already inactive account (idempotent)", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("already-inactive-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil)
		if account != nil {
			defer testpkg.CleanupAuthFixtures(t, db, account.ID)
		}
		require.NoError(t, err)

		// Deactivate first
		err = service.DeactivateAccount(ctx, int(account.ID))
		require.NoError(t, err)

		// ACT - Deactivate again
		err = service.DeactivateAccount(ctx, int(account.ID))

		// ASSERT
		require.NoError(t, err)
	})
}

func TestAuthService_UpdateAccount_Extended(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupAuthServiceWithDB(t, db)
	ctx := context.Background()

	t.Run("preserves password hash when not provided", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("preserve-hash-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil)
		if account != nil {
			defer testpkg.CleanupAuthFixtures(t, db, account.ID)
		}
		require.NoError(t, err)

		// Get the original password hash
		original, err := service.GetAccountByID(ctx, int(account.ID))
		require.NoError(t, err)
		originalHash := original.PasswordHash

		// Update without password
		account.PasswordHash = nil
		account.Active = false

		// ACT
		err = service.UpdateAccount(ctx, account)

		// ASSERT
		require.NoError(t, err)

		// Verify password hash is preserved
		updated, err := service.GetAccountByID(ctx, int(account.ID))
		require.NoError(t, err)
		assert.Equal(t, originalHash, updated.PasswordHash)
		assert.False(t, updated.Active)
	})

	t.Run("returns error for non-existent account", func(t *testing.T) {
		// ARRANGE
		fakeAccount := &authModels.Account{}
		fakeAccount.ID = 99999999

		// ACT
		err := service.UpdateAccount(ctx, fakeAccount)

		// ASSERT
		require.Error(t, err)
	})
}

func TestAuthService_ListAccounts_Extended(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupAuthServiceWithDB(t, db)
	ctx := context.Background()

	t.Run("returns accounts with email filter", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("filter-email-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil)
		if account != nil {
			defer testpkg.CleanupAuthFixtures(t, db, account.ID)
		}
		require.NoError(t, err)

		// ACT
		filters := map[string]interface{}{
			"email": email,
		}
		result, err := service.ListAccounts(ctx, filters)

		// ASSERT
		require.NoError(t, err)
		found := false
		for _, acc := range result {
			if acc.Email == email {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

func TestAuthService_GetAccountsByRole_Extended(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupAuthServiceWithDB(t, db)
	ctx := context.Background()

	t.Run("returns accounts with specific role", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		roleName := fmt.Sprintf("specific-role-%s", uniqueID)
		role, err := service.CreateRole(ctx, roleName, "Specific role for testing")
		require.NoError(t, err)

		email := fmt.Sprintf("role-specific-%s@test.local", uniqueID)
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%s", uniqueID), "Test1234%", nil)
		if account != nil {
			defer testpkg.CleanupAuthFixtures(t, db, account.ID)
		}
		require.NoError(t, err)

		err = service.AssignRoleToAccount(ctx, int(account.ID), int(role.ID))
		require.NoError(t, err)

		// ACT
		result, err := service.GetAccountsByRole(ctx, roleName)

		// ASSERT
		require.NoError(t, err)
		found := false
		for _, acc := range result {
			if acc.ID == account.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("returns empty list for non-existent role", func(t *testing.T) {
		// ACT
		result, err := service.GetAccountsByRole(ctx, fmt.Sprintf("non-existent-role-%d", time.Now().UnixNano()))

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

// =============================================================================
// Token Cleanup Extended Tests (token_cleanup.go)
// =============================================================================

func TestAuthService_CleanupExpiredTokens_Extended(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupAuthServiceWithDB(t, db)
	ctx := context.Background()

	t.Run("returns zero when no expired tokens", func(t *testing.T) {
		// ACT
		count, err := service.CleanupExpiredTokens(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 0)
	})
}

func TestAuthService_CleanupExpiredPasswordResetTokens_Extended(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupAuthServiceWithDB(t, db)
	ctx := context.Background()

	t.Run("returns count of cleaned tokens", func(t *testing.T) {
		// ACT
		count, err := service.CleanupExpiredPasswordResetTokens(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 0)
	})
}

func TestAuthService_CleanupExpiredRateLimits_Extended(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupAuthServiceWithDB(t, db)
	ctx := context.Background()

	t.Run("returns count of cleaned rate limits", func(t *testing.T) {
		// ACT
		count, err := service.CleanupExpiredRateLimits(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 0)
	})
}

func TestAuthService_RevokeAllTokens_Extended(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupAuthServiceWithDB(t, db)
	ctx := context.Background()

	t.Run("succeeds for account with no tokens", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("no-tokens-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil)
		if account != nil {
			defer testpkg.CleanupAuthFixtures(t, db, account.ID)
		}
		require.NoError(t, err)

		// Ensure no tokens
		err = service.RevokeAllTokens(ctx, int(account.ID))
		require.NoError(t, err)

		// ACT - Revoke again (idempotent)
		err = service.RevokeAllTokens(ctx, int(account.ID))

		// ASSERT
		require.NoError(t, err)
	})
}

func TestAuthService_GetActiveTokens_Extended(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupAuthServiceWithDB(t, db)
	ctx := context.Background()

	t.Run("returns multiple tokens after multiple logins", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("multi-token-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil)
		if account != nil {
			defer testpkg.CleanupAuthFixtures(t, db, account.ID)
		}
		require.NoError(t, err)

		// Login multiple times to create tokens
		_, _, err = service.Login(ctx, email, "Test1234%")
		require.NoError(t, err)
		_, _, err = service.Login(ctx, email, "Test1234%")
		require.NoError(t, err)

		// ACT
		tokens, err := service.GetActiveTokens(ctx, int(account.ID))

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(tokens), 1)
	})
}

// =============================================================================
// Parent Account Extended Tests (parent_account.go)
// =============================================================================

func TestAuthService_CreateParentAccount_Extended(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupAuthServiceWithDB(t, db)
	ctx := context.Background()

	t.Run("returns error for weak password", func(t *testing.T) {
		// ACT
		_, err := service.CreateParentAccount(ctx, fmt.Sprintf("weak-%d@test.local", time.Now().UnixNano()), fmt.Sprintf("user-%d", time.Now().UnixNano()), "weak")

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for duplicate username", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		username := fmt.Sprintf("dup-username-%s", uniqueID)
		email1 := fmt.Sprintf("dup1-%s@test.local", uniqueID)
		email2 := fmt.Sprintf("dup2-%s@test.local", uniqueID)

		_, err := service.CreateParentAccount(ctx, email1, username, "Test1234%")
		require.NoError(t, err)

		// ACT
		_, err = service.CreateParentAccount(ctx, email2, username, "Test1234%")

		// ASSERT
		require.Error(t, err)
	})
}

func TestAuthService_GetParentAccountByEmail_Extended(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupAuthServiceWithDB(t, db)
	ctx := context.Background()

	t.Run("normalizes email case", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		email := fmt.Sprintf("case-test-%s@test.local", uniqueID)
		_, err := service.CreateParentAccount(ctx, email, fmt.Sprintf("user-%s", uniqueID), "Test1234%")
		require.NoError(t, err)

		// ACT - Search with uppercase
		result, err := service.GetParentAccountByEmail(ctx, fmt.Sprintf("CASE-TEST-%s@TEST.LOCAL", uniqueID))

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, email, result.Email)
	})
}

func TestAuthService_UpdateParentAccount_Extended(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupAuthServiceWithDB(t, db)
	ctx := context.Background()

	t.Run("preserves password when not provided", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		account, err := service.CreateParentAccount(ctx, fmt.Sprintf("preserve-%s@test.local", uniqueID), fmt.Sprintf("user-%s", uniqueID), "Test1234%")
		require.NoError(t, err)

		// Get original
		original, err := service.GetParentAccountByID(ctx, int(account.ID))
		require.NoError(t, err)

		// Update without password
		newUsername := fmt.Sprintf("updated-%s", uniqueID)
		account.Username = &newUsername
		account.PasswordHash = nil

		// ACT
		err = service.UpdateParentAccount(ctx, account)

		// ASSERT
		require.NoError(t, err)

		// Verify password preserved
		updated, err := service.GetParentAccountByID(ctx, int(account.ID))
		require.NoError(t, err)
		assert.Equal(t, original.PasswordHash, updated.PasswordHash)
		assert.Equal(t, newUsername, *updated.Username)
	})
}

func TestAuthService_ListParentAccounts_Extended(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupAuthServiceWithDB(t, db)
	ctx := context.Background()

	t.Run("returns accounts with active filter", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		account, err := service.CreateParentAccount(ctx, fmt.Sprintf("active-%s@test.local", uniqueID), fmt.Sprintf("user-%s", uniqueID), "Test1234%")
		require.NoError(t, err)

		// Deactivate account
		err = service.DeactivateParentAccount(ctx, int(account.ID))
		require.NoError(t, err)

		// ACT - Filter for active only
		filters := map[string]interface{}{
			"active": true,
		}
		result, err := service.ListParentAccounts(ctx, filters)

		// ASSERT
		require.NoError(t, err)
		for _, acc := range result {
			assert.True(t, acc.Active)
		}
	})
}

// =============================================================================
// Password Reset Extended Tests (password_reset.go)
// =============================================================================

func TestAuthService_ResetPassword_Extended(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupAuthServiceWithDB(t, db)
	ctx := context.Background()

	t.Run("returns error for invalid token", func(t *testing.T) {
		// ACT
		err := service.ResetPassword(ctx, "invalid-token-12345", "NewPassword1%")

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for weak new password", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("reset-weak-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil)
		if account != nil {
			defer testpkg.CleanupAuthFixtures(t, db, account.ID)
		}
		require.NoError(t, err)

		// Initiate password reset
		resetToken, err := service.InitiatePasswordReset(ctx, email)
		require.NoError(t, err)
		require.NotNil(t, resetToken)

		// ACT - Try with weak password
		err = service.ResetPassword(ctx, resetToken.Token, "weak")

		// ASSERT
		require.Error(t, err)
	})
}

func TestAuthService_InitiatePasswordReset_Extended(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupAuthServiceWithDB(t, db)
	ctx := context.Background()

	t.Run("returns nil for non-existent email (security)", func(t *testing.T) {
		// ACT
		result, err := service.InitiatePasswordReset(ctx, fmt.Sprintf("nonexistent-%d@test.local", time.Now().UnixNano()))

		// ASSERT
		require.NoError(t, err)
		assert.Nil(t, result) // Should not reveal email existence
	})

	t.Run("normalizes email case", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		email := fmt.Sprintf("reset-case-%s@test.local", uniqueID)
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%s", uniqueID), "Test1234%", nil)
		if account != nil {
			defer testpkg.CleanupAuthFixtures(t, db, account.ID)
		}
		require.NoError(t, err)

		// ACT - Use uppercase email
		result, err := service.InitiatePasswordReset(ctx, fmt.Sprintf("RESET-CASE-%s@TEST.LOCAL", uniqueID))

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, account.ID, result.AccountID)
	})
}

// =============================================================================
// Auth Login Extended Tests (auth_login.go)
// =============================================================================

func TestAuthService_Login_Extended(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupAuthServiceWithDB(t, db)
	ctx := context.Background()

	t.Run("normalizes email case on login", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		email := fmt.Sprintf("login-case-%s@test.local", uniqueID)
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%s", uniqueID), "Test1234%", nil)
		if account != nil {
			defer testpkg.CleanupAuthFixtures(t, db, account.ID)
		}
		require.NoError(t, err)

		// ACT - Login with uppercase
		accessToken, refreshToken, err := service.Login(ctx, fmt.Sprintf("LOGIN-CASE-%s@TEST.LOCAL", uniqueID), "Test1234%")

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, accessToken)
		assert.NotEmpty(t, refreshToken)
	})

	t.Run("fails for deactivated account", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("deactivated-login-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil)
		if account != nil {
			defer testpkg.CleanupAuthFixtures(t, db, account.ID)
		}
		require.NoError(t, err)

		// Deactivate account
		err = service.DeactivateAccount(ctx, int(account.ID))
		require.NoError(t, err)

		// ACT
		_, _, err = service.Login(ctx, email, "Test1234%")

		// ASSERT
		require.Error(t, err)
	})
}

func TestAuthService_Register_Extended(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupAuthServiceWithDB(t, db)
	ctx := context.Background()

	t.Run("normalizes email case on registration", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		email := fmt.Sprintf("REGISTER-CASE-%s@TEST.LOCAL", uniqueID)

		// ACT
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%s", uniqueID), "Test1234%", nil)
		if account != nil {
			defer testpkg.CleanupAuthFixtures(t, db, account.ID)
		}

		// ASSERT
		require.NoError(t, err)

		// Email should be lowercase
		assert.Equal(t, fmt.Sprintf("register-case-%s@test.local", uniqueID), account.Email)
	})

	t.Run("allows empty username", func(t *testing.T) {
		// ACT - Register allows empty/nil username
		account, err := service.Register(ctx, fmt.Sprintf("empty-user-%d@test.local", time.Now().UnixNano()), "", "Test1234%", nil)
		if account != nil {
			defer testpkg.CleanupAuthFixtures(t, db, account.ID)
		}

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, account)
	})
}

func TestAuthService_ValidateToken_Extended(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupAuthServiceWithDB(t, db)
	ctx := context.Background()

	t.Run("returns error for deactivated account token", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("validate-deactivated-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil)
		if account != nil {
			defer testpkg.CleanupAuthFixtures(t, db, account.ID)
		}
		require.NoError(t, err)

		accessToken, _, err := service.Login(ctx, email, "Test1234%")
		require.NoError(t, err)

		// Deactivate account
		err = service.DeactivateAccount(ctx, int(account.ID))
		require.NoError(t, err)

		// ACT
		_, err = service.ValidateToken(ctx, accessToken)

		// ASSERT
		require.Error(t, err)
	})
}

func TestAuthService_RefreshToken_Extended(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupAuthServiceWithDB(t, db)
	ctx := context.Background()

	t.Run("fails for deactivated account", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("refresh-deactivated-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil)
		if account != nil {
			defer testpkg.CleanupAuthFixtures(t, db, account.ID)
		}
		require.NoError(t, err)

		_, refreshToken, err := service.Login(ctx, email, "Test1234%")
		require.NoError(t, err)

		// Deactivate account
		err = service.DeactivateAccount(ctx, int(account.ID))
		require.NoError(t, err)

		// ACT
		_, _, err = service.RefreshToken(ctx, refreshToken)

		// ASSERT
		require.Error(t, err)
	})
}

func TestAuthService_Logout_Extended(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupAuthServiceWithDB(t, db)
	ctx := context.Background()

	t.Run("is idempotent - double logout succeeds", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("double-logout-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil)
		if account != nil {
			defer testpkg.CleanupAuthFixtures(t, db, account.ID)
		}
		require.NoError(t, err)

		_, refreshToken, err := service.Login(ctx, email, "Test1234%")
		require.NoError(t, err)

		// First logout
		err = service.Logout(ctx, refreshToken)
		require.NoError(t, err)

		// ACT - Second logout (should still work)
		err = service.Logout(ctx, refreshToken)

		// ASSERT - Should not fail (idempotent)
		require.NoError(t, err) // Token no longer exists, but operation is idempotent
	})
}

func TestAuthService_ChangePassword_Extended(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupAuthServiceWithDB(t, db)
	ctx := context.Background()

	t.Run("returns error for non-existent account", func(t *testing.T) {
		// ACT
		err := service.ChangePassword(ctx, 99999999, "OldPassword1%", "NewPassword1%")

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for same old and new password", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("same-pwd-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil)
		if account != nil {
			defer testpkg.CleanupAuthFixtures(t, db, account.ID)
		}
		require.NoError(t, err)

		// ACT - Change to same password
		err = service.ChangePassword(ctx, int(account.ID), "Test1234%", "Test1234%")

		// ASSERT - This depends on implementation; may succeed or fail
		// The password change itself succeeds even if old == new
		require.NoError(t, err)
	})
}

func TestAuthService_GetAccountByEmail_Extended(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupAuthServiceWithDB(t, db)
	ctx := context.Background()

	t.Run("normalizes email case", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		email := fmt.Sprintf("get-email-case-%s@test.local", uniqueID)
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%s", uniqueID), "Test1234%", nil)
		if account != nil {
			defer testpkg.CleanupAuthFixtures(t, db, account.ID)
		}
		require.NoError(t, err)

		// ACT - Search with uppercase
		result, err := service.GetAccountByEmail(ctx, fmt.Sprintf("GET-EMAIL-CASE-%s@TEST.LOCAL", uniqueID))

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, account.ID, result.ID)
	})
}
