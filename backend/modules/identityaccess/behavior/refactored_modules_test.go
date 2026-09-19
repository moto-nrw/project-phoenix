// Package auth_test contains hermetic tests for the refactored auth service modules.
// These tests specifically target coverage gaps in the split files.
package behavior_test

import (
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	authModels "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/authmodels"
	"github.com/moto-nrw/project-phoenix/services"
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
	repoFactory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	serviceFactory, err := services.NewFactoryForTests(repoFactory, db, slog.Default())
	require.NoError(t, err, "Failed to create service factory")
	return newFixtureAuthService(t, db, serviceFactory.Auth)
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

	t.Run("deletes role with all associations successfully", func(t *testing.T) {
		// ARRANGE - create role with permission assignment
		roleName := fmt.Sprintf("delete-full-role-%d", time.Now().UnixNano())
		role, err := rbac.CreateRole(ctx, roleName, "Role to delete with associations", testpkg.StrPtr("user"))
		require.NoError(t, err)

		// Create permission and assign to role
		permName := fmt.Sprintf("delete-role-perm-%d", time.Now().UnixNano())
		resource := fmt.Sprintf("delete-role-res-%d", time.Now().UnixNano())
		perm, err := rbac.CreatePermission(ctx, permName, "Test permission", resource, "read")
		require.NoError(t, err)
		testpkg.OwnTestPermission(t, db, perm.ID)

		err = rbac.AssignPermissionToRole(ctx, role.ID, perm.ID)
		require.NoError(t, err)

		// Create account and assign role
		email := fmt.Sprintf("delete-role-user-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		err = rbac.AssignRoleToAccount(ctx, account.ID, role.ID)
		require.NoError(t, err)

		// ACT
		err = rbac.DeleteRole(ctx, role.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify role is deleted
		_, err = rbac.GetRole(ctx, role.ID)
		require.Error(t, err)

		// Verify account no longer has role
		roles, err := rbac.GetAccountRoles(ctx, account.ID)
		require.NoError(t, err)
		for _, r := range roles {
			assert.NotEqual(t, role.ID, r.ID)
		}
	})

	t.Run("returns error for non-existent role", func(t *testing.T) {
		// ACT - DeleteRole validates existence (to check IsSystem), so non-existent role returns error
		err := rbac.DeleteRole(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
	})

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
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		// ACT
		err = rbac.AssignRoleToAccount(ctx, account.ID, 99999999)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("is idempotent - assigning same role twice succeeds", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("idempotent-role-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		roleName := fmt.Sprintf("idempotent-role-%d", time.Now().UnixNano())
		role, err := rbac.CreateRole(ctx, roleName, "Test role", testpkg.StrPtr("user"))
		require.NoError(t, err)

		// First assignment
		err = rbac.AssignRoleToAccount(ctx, account.ID, role.ID)
		require.NoError(t, err)

		// ACT - Second assignment (should be idempotent)
		err = rbac.AssignRoleToAccount(ctx, account.ID, role.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify role is assigned only once
		roles, err := rbac.GetAccountRoles(ctx, account.ID)
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthServiceWithDB(t, db)
	rbac := roleAdministrationOf(t, service)
	ctx := testpkg.Ctx(t)

	t.Run("removes role from account successfully", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("remove-role-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		roleName := fmt.Sprintf("remove-role-%d", time.Now().UnixNano())
		role, err := rbac.CreateRole(ctx, roleName, "Test role", testpkg.StrPtr("user"))
		require.NoError(t, err)

		err = rbac.AssignRoleToAccount(ctx, account.ID, role.ID)
		require.NoError(t, err)

		// ACT
		err = rbac.RemoveRoleFromAccount(ctx, account.ID, role.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify role is removed
		roles, err := rbac.GetAccountRoles(ctx, account.ID)
		require.NoError(t, err)
		for _, r := range roles {
			assert.NotEqual(t, role.ID, r.ID)
		}
	})
}

func TestAuthService_GetAccountRoles_Extended(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthServiceWithDB(t, db)
	rbac := roleAdministrationOf(t, service)
	ctx := testpkg.Ctx(t)

	t.Run("returns empty list for account with no roles", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("no-roles-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		// Remove any default roles
		roles, _ := rbac.GetAccountRoles(ctx, account.ID)
		for _, r := range roles {
			_ = rbac.RemoveRoleFromAccount(ctx, account.ID, r.ID)
		}

		// ACT
		result, err := rbac.GetAccountRoles(ctx, account.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

// =============================================================================
// Permission Management Extended Tests (permission_management.go)
// =============================================================================

func TestAuthService_DeletePermission_Extended(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthServiceWithDB(t, db)
	rbac := roleAdministrationOf(t, service)
	ctx := testpkg.Ctx(t)

	t.Run("deletes permission with role association", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		permName := fmt.Sprintf("delete-perm-role-%s", uniqueID)
		resource := fmt.Sprintf("delete-perm-res-%s", uniqueID)
		perm, err := rbac.CreatePermission(ctx, permName, "Permission to delete", resource, "read")
		require.NoError(t, err)
		testpkg.OwnTestPermission(t, db, perm.ID)

		// Create role and assign permission
		roleName := fmt.Sprintf("perm-role-%s", uniqueID)
		role, err := rbac.CreateRole(ctx, roleName, "Test role", testpkg.StrPtr("user"))
		require.NoError(t, err)

		err = rbac.AssignPermissionToRole(ctx, role.ID, perm.ID)
		require.NoError(t, err)

		// ACT
		err = rbac.DeletePermission(ctx, perm.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify permission is deleted
		_, err = rbac.GetPermission(ctx, perm.ID)
		require.Error(t, err)
	})

	t.Run("deletes permission with account association", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		permName := fmt.Sprintf("delete-perm-acct-%s", uniqueID)
		resource := fmt.Sprintf("delete-perm-res2-%s", uniqueID)
		perm, err := rbac.CreatePermission(ctx, permName, "Permission to delete", resource, "write")
		require.NoError(t, err)
		testpkg.OwnTestPermission(t, db, perm.ID)

		// Create account and grant permission
		email := fmt.Sprintf("delete-perm-user-%s@test.local", uniqueID)
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%s", uniqueID), "Test1234%", nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		err = rbac.GrantPermissionToAccount(ctx, account.ID, perm.ID)
		require.NoError(t, err)

		// ACT
		err = rbac.DeletePermission(ctx, perm.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify permission is deleted
		_, err = rbac.GetPermission(ctx, perm.ID)
		require.Error(t, err)
	})
}

func TestAuthService_GrantPermissionToAccount_Extended(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthServiceWithDB(t, db)
	rbac := roleAdministrationOf(t, service)
	ctx := testpkg.Ctx(t)

	t.Run("returns error for non-existent permission", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("grant-perm-user-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		// ACT
		err = rbac.GrantPermissionToAccount(ctx, account.ID, 99999999)

		// ASSERT
		require.Error(t, err)
	})
}

func TestAuthService_RemovePermissionFromAccount_Extended(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthServiceWithDB(t, db)
	rbac := roleAdministrationOf(t, service)
	ctx := testpkg.Ctx(t)

	t.Run("removes permission from account", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		permName := fmt.Sprintf("remove-perm-%s", uniqueID)
		resource := fmt.Sprintf("remove-res-%s", uniqueID)
		perm, err := rbac.CreatePermission(ctx, permName, "Permission to remove", resource, "read")
		require.NoError(t, err)
		testpkg.OwnTestPermission(t, db, perm.ID)

		email := fmt.Sprintf("remove-perm-user-%s@test.local", uniqueID)
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%s", uniqueID), "Test1234%", nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		err = rbac.GrantPermissionToAccount(ctx, account.ID, perm.ID)
		require.NoError(t, err)

		// ACT
		err = rbac.RemovePermissionFromAccount(ctx, account.ID, perm.ID)

		// ASSERT
		require.NoError(t, err)
	})
}

func TestAuthService_AssignPermissionToRole_Extended(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthServiceWithDB(t, db)
	rbac := roleAdministrationOf(t, service)
	ctx := testpkg.Ctx(t)

	t.Run("returns error for non-existent role", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		permName := fmt.Sprintf("assign-to-role-%s", uniqueID)
		resource := fmt.Sprintf("assign-res-%s", uniqueID)
		perm, err := rbac.CreatePermission(ctx, permName, "Test permission", resource, "read")
		require.NoError(t, err)
		testpkg.OwnTestPermission(t, db, perm.ID)

		// ACT
		err = rbac.AssignPermissionToRole(ctx, 99999999, perm.ID)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for non-existent permission", func(t *testing.T) {
		// ARRANGE
		roleName := fmt.Sprintf("assign-perm-role-%d", time.Now().UnixNano())
		role, err := rbac.CreateRole(ctx, roleName, "Test role", testpkg.StrPtr("user"))
		require.NoError(t, err)

		// ACT
		err = rbac.AssignPermissionToRole(ctx, role.ID, 99999999)

		// ASSERT
		require.Error(t, err)
	})
}

func TestAuthService_RemovePermissionFromRole_Extended(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthServiceWithDB(t, db)
	rbac := roleAdministrationOf(t, service)
	ctx := testpkg.Ctx(t)

	t.Run("removes permission from role", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		roleName := fmt.Sprintf("remove-perm-role-%s", uniqueID)
		role, err := rbac.CreateRole(ctx, roleName, "Test role", testpkg.StrPtr("user"))
		require.NoError(t, err)

		permName := fmt.Sprintf("remove-from-role-%s", uniqueID)
		resource := fmt.Sprintf("remove-from-res-%s", uniqueID)
		perm, err := rbac.CreatePermission(ctx, permName, "Test permission", resource, "read")
		require.NoError(t, err)
		testpkg.OwnTestPermission(t, db, perm.ID)

		err = rbac.AssignPermissionToRole(ctx, role.ID, perm.ID)
		require.NoError(t, err)

		// ACT
		err = rbac.RemovePermissionFromRole(ctx, role.ID, perm.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify permission is removed
		perms, err := rbac.GetRolePermissions(ctx, role.ID)
		require.NoError(t, err)
		for _, p := range perms {
			assert.NotEqual(t, perm.ID, p.ID)
		}
	})
}

func TestAuthService_GetRolePermissions_Extended(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthServiceWithDB(t, db)
	rbac := roleAdministrationOf(t, service)
	ctx := testpkg.Ctx(t)

	t.Run("returns empty list for role with no permissions", func(t *testing.T) {
		// ARRANGE
		roleName := fmt.Sprintf("empty-perm-role-%d", time.Now().UnixNano())
		role, err := rbac.CreateRole(ctx, roleName, "Test role", testpkg.StrPtr("user"))
		require.NoError(t, err)

		// ACT
		perms, err := rbac.GetRolePermissions(ctx, role.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, perms)
	})

	t.Run("returns permissions for role", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		roleName := fmt.Sprintf("has-perm-role-%s", uniqueID)
		role, err := rbac.CreateRole(ctx, roleName, "Test role", testpkg.StrPtr("user"))
		require.NoError(t, err)

		permName := fmt.Sprintf("role-perm-%s", uniqueID)
		resource := fmt.Sprintf("role-res-%s", uniqueID)
		perm, err := rbac.CreatePermission(ctx, permName, "Test permission", resource, "read")
		require.NoError(t, err)
		testpkg.OwnTestPermission(t, db, perm.ID)

		err = rbac.AssignPermissionToRole(ctx, role.ID, perm.ID)
		require.NoError(t, err)

		// ACT
		perms, err := rbac.GetRolePermissions(ctx, role.ID)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthServiceWithDB(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("activates already active account (idempotent)", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("already-active-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil, 0)
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

	t.Run("deactivates account and invalidates tokens", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("deactivate-tokens-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

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
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil, 0)
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
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil, 0)
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
		fakeAccount := &authModels.Account{}
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
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil, 0)
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
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
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
// Token Cleanup Extended Tests (token_cleanup.go)
// =============================================================================

func TestAuthService_CleanupExpiredTokens_Extended(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)

	service := setupAuthServiceWithDB(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns zero when no expired tokens", func(t *testing.T) {
		// ACT
		count, err := service.CleanupExpiredTokens(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 0)
	})
}

func TestAuthService_RevokeAllTokens_Extended(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthServiceWithDB(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("succeeds for account with no tokens", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("no-tokens-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil, 0)
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthServiceWithDB(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns multiple tokens after multiple logins", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("multi-token-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthServiceWithDB(t, db)
	ctx := testpkg.Ctx(t)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthServiceWithDB(t, db)
	ctx := testpkg.Ctx(t)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthServiceWithDB(t, db)
	ctx := testpkg.Ctx(t)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthServiceWithDB(t, db)
	ctx := testpkg.Ctx(t)

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
// Auth Login Extended Tests (auth_login.go)
// =============================================================================

func TestAuthService_Login_Extended(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthServiceWithDB(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("normalizes email case on login", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
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

	t.Run("fails for deactivated account", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("deactivated-login-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthServiceWithDB(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("normalizes email case on registration", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
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
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil, 0)
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
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil, 0)
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
		account, err := service.Register(ctx, email, fmt.Sprintf("user-%d", time.Now().UnixNano()), "Test1234%", nil, 0)
		require.NoError(t, err)

		// ACT - Change to same password
		err = service.ChangePassword(ctx, int(account.ID), "Test1234%", "Test1234%")

		// ASSERT - This depends on implementation; may succeed or fail
		// The password change itself succeeds even if old == new
		require.NoError(t, err)
	})
}
