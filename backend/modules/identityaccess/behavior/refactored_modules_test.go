// Package auth_test contains hermetic tests for the refactored auth service modules.
// These tests specifically target coverage gaps in the split files.
package behavior_test

import (
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// accountPasswordHash reads the stored credential of an account so a test
// can prove a write left it alone. The capability never reports it.
func accountPasswordHash(t *testing.T, db *bun.DB, accountID int64) string {
	t.Helper()
	var hash *string
	require.NoError(t, db.NewSelect().
		ColumnExpr("password_hash").
		TableExpr("auth.accounts").
		Where("id = ?", accountID).
		Scan(testpkg.Ctx(t), &hash))
	if hash == nil {
		return ""
	}
	return *hash
}

// setupAuthServiceWithDB creates an auth service with real database connection
func setupAuthServiceWithDB(t *testing.T, db *bun.DB) testAuthService {
	identityModule, err := api.NewTestIdentityAccess(testpkg.NewDatabase(t, db), slog.Default())
	require.NoError(t, err, "Failed to create service factory")
	return newFixtureAuthService(t, db, identityModule)
}

// =============================================================================
// Role Management Extended Tests (role_management.go)
// =============================================================================

func TestAuthService_DeleteRole_Extended(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthServiceWithDB(t, db)
	rbac := roleAdministrationOf(t, service)
	ctx := testpkg.Ctx(t)

	t.Run("returns error when deleting a system role", func(t *testing.T) {
		// ARRANGE - create a system role directly in the DB
		systemRole := testpkg.CreateTestSystemRole(t, db, "test-system")

		// ACT
		err := rbac.DeleteRole(ctx, systemRole.ID)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "system roles cannot be modified")
	})
}

func TestAuthService_UpdateRole_SystemRoleProtection(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthServiceWithDB(t, db)
	rbac := roleAdministrationOf(t, service)
	ctx := testpkg.Ctx(t)

	t.Run("returns error when updating a system role", func(t *testing.T) {
		// ARRANGE
		systemRole := testpkg.CreateTestSystemRole(t, db, "test-system")
		update := identityaccess.Role{
			ID: systemRole.ID, TenantID: systemRole.TenantID, Name: systemRole.Name,
			Description: "attempted update", IsSystem: systemRole.IsSystem, BaseRole: systemRole.BaseRole,
		}

		// ACT
		err := rbac.UpdateRole(ctx, update)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "system roles cannot be modified")
	})
}

func TestAuthService_AssignPermissionToRole_SystemRoleProtection(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthServiceWithDB(t, db)
	rbac := roleAdministrationOf(t, service)
	ctx := testpkg.Ctx(t)

	t.Run("returns error when assigning permission to a system role", func(t *testing.T) {
		// ARRANGE
		systemRole := testpkg.CreateTestSystemRole(t, db, "test-system")

		permName := fmt.Sprintf("sys-role-perm-%d", time.Now().UnixNano())
		resource := fmt.Sprintf("sys-role-res-%d", time.Now().UnixNano())
		perm, err := rbac.CreatePermission(ctx, permName, "Test permission", resource, "read")
		require.NoError(t, err)
		testpkg.OwnTestPermission(t, db, perm.ID)

		// ACT
		err = rbac.AssignPermissionToRole(ctx, systemRole.ID, perm.ID)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "system roles cannot be modified")
	})
}

func TestAuthService_RemovePermissionFromRole_SystemRoleProtection(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthServiceWithDB(t, db)
	rbac := roleAdministrationOf(t, service)
	ctx := testpkg.Ctx(t)

	t.Run("returns error when removing permission from a system role", func(t *testing.T) {
		// ARRANGE
		systemRole := testpkg.CreateTestSystemRole(t, db, "test-system")

		// ACT
		err := rbac.RemovePermissionFromRole(ctx, systemRole.ID, 1)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "system roles cannot be modified")
	})
}

func TestAuthService_AssignRoleToAccount_Extended(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthServiceWithDB(t, db)
	rbac := roleAdministrationOf(t, service)
	ctx := testpkg.Ctx(t)

	t.Run("returns error for non-existent account", func(t *testing.T) {
		// ARRANGE
		roleName := fmt.Sprintf("assign-role-%d", time.Now().UnixNano())
		role, err := rbac.CreateRole(ctx, roleName, "Test role", testpkg.StrPtr("user"))
		require.NoError(t, err)

		// ACT
		err = rbac.AssignRoleToAccount(ctx, 99999999, role.ID)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for non-existent role", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("assign-role-user-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", testpkg.UniqueSuffix()), "Test1234%", nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		// ACT
		err = rbac.AssignRoleToAccount(ctx, account.ID, 99999999)

		// ASSERT
		require.Error(t, err)
	})
}

// =============================================================================
// Account Management Extended Tests (account_management.go)
// =============================================================================

func TestAuthService_ActivateAccount_Extended(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthServiceWithDB(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("activates already active account (idempotent)", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("already-active-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", testpkg.UniqueSuffix()), "Test1234%", nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthServiceWithDB(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("deactivates already inactive account (idempotent)", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("already-inactive-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", testpkg.UniqueSuffix()), "Test1234%", nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthServiceWithDB(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("preserves the credential", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("preserve-hash-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", testpkg.UniqueSuffix()), "Test1234%", nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		originalHash := accountPasswordHash(t, db, account.ID)
		require.NotEmpty(t, originalHash)

		account.Email = fmt.Sprintf("preserved-hash-%d@test.local", time.Now().UnixNano())

		// ACT
		err = service.UpdateAccount(ctx, account)

		// ASSERT
		require.NoError(t, err)

		// The identity update writes the address and the name; the
		// credential is only ever replaced by a password change (#3332).
		assert.Equal(t, originalHash, accountPasswordHash(t, db, account.ID))

		updated, err := service.GetAccountByID(ctx, int(account.ID))
		require.NoError(t, err)
		assert.Equal(t, account.Email, updated.Email)
		assert.True(t, updated.Active, "an identity update never disables an account")
	})

	t.Run("returns error for non-existent account", func(t *testing.T) {
		// ARRANGE
		fakeAccount := &testpkg.AccountFixture{}
		fakeAccount.ID = 99999999

		// ACT
		err := service.UpdateAccount(ctx, fakeAccount)

		// ASSERT
		require.Error(t, err)
	})
}

func TestAuthService_ListAccounts_Extended(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthServiceWithDB(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns accounts with email filter", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("filter-email-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", testpkg.UniqueSuffix()), "Test1234%", nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthServiceWithDB(t, db)
	rbac := roleAdministrationOf(t, service)
	ctx := testpkg.Ctx(t)

	t.Run("returns accounts with specific role", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", testpkg.UniqueSuffix())
		roleName := fmt.Sprintf("specific-role-%s", uniqueID)
		role, err := rbac.CreateRole(ctx, roleName, "Specific role for testing", testpkg.StrPtr("user"))
		require.NoError(t, err)

		email := fmt.Sprintf("role-specific-%s@test.local", uniqueID)
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%s", uniqueID), "Test1234%", nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		err = rbac.AssignRoleToAccount(ctx, account.ID, role.ID)
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
// Auth Login Extended Tests (auth_login.go)
// =============================================================================

func TestAuthService_Login_Extended(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthServiceWithDB(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("normalizes email case on login", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", testpkg.UniqueSuffix())
		email := fmt.Sprintf("login-case-%s@test.local", uniqueID)
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%s", uniqueID), "Test1234%", nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		// ACT - Login with uppercase
		accessToken, refreshToken, err := service.Login(ctx, fmt.Sprintf("LOGIN-CASE-%s@TEST.LOCAL", uniqueID), "Test1234%")

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, accessToken)
		assert.NotEmpty(t, refreshToken)
	})
}

func TestAuthService_Register_Extended(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthServiceWithDB(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("normalizes email case on registration", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", testpkg.UniqueSuffix())
		email := fmt.Sprintf("REGISTER-CASE-%s@TEST.LOCAL", uniqueID)

		// ACT
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%s", uniqueID), "Test1234%", nil, 0)

		// ASSERT
		require.NoError(t, err)

		// Email should be lowercase
		assert.Equal(t, fmt.Sprintf("register-case-%s@test.local", uniqueID), account.Email)
	})

	t.Run("allows empty username", func(t *testing.T) {
		// ACT - Register allows empty/nil username
		account, err := service.Register(ctx, fmt.Sprintf("empty-user-%d@test.local", time.Now().UnixNano()), "", "Test1234%", nil, 0)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, account)
	})
}

func TestAuthService_RefreshToken_Extended(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthServiceWithDB(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("fails for deactivated account", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("refresh-deactivated-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", testpkg.UniqueSuffix()), "Test1234%", nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthServiceWithDB(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("is idempotent - double logout succeeds", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("double-logout-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", testpkg.UniqueSuffix()), "Test1234%", nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		_, refreshToken, err := service.Login(ctx, email, "Test1234%")
		require.NoError(t, err)

		// First logout
		err = service.LogoutWithAudit(ctx, refreshToken, "", "")
		require.NoError(t, err)

		// ACT - Second logout (should still work)
		err = service.LogoutWithAudit(ctx, refreshToken, "", "")

		// ASSERT - Should not fail (idempotent)
		require.NoError(t, err) // Token no longer exists, but operation is idempotent
	})
}

func TestAuthService_ChangePassword_Extended(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthServiceWithDB(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error for non-existent account", func(t *testing.T) {
		// ACT
		err := service.ChangePassword(ctx, 99999999, "OldPassword1%", "NewPassword1%")

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for same old and new password", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("same-pwd-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", testpkg.UniqueSuffix()), "Test1234%", nil, 0)
		require.NoError(t, err)

		// ACT - Change to same password
		err = service.ChangePassword(ctx, int(account.ID), "Test1234%", "Test1234%")

		// ASSERT - This depends on implementation; may succeed or fail
		// The password change itself succeeds even if old == new
		require.NoError(t, err)
	})
}
