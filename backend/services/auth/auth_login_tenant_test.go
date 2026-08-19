// Package auth_test contains integration tests for tenant resolution during login.
// These tests exercise LoginWithAudit with explicit tenant slugs (resolveAccountTenantBySlug)
// and the empty-slug default path (resolveAccountTenantDefault).
package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/services/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// LoginWithAudit Tenant Resolution Tests
// =============================================================================

func TestLoginWithAudit_ValidTenantSlug(t *testing.T) {
	// LoginWithAudit with a valid tenant slug should succeed when the account
	// is mapped to that tenant. This covers the resolveAccountTenantBySlug
	// success path, ensureAccountRolesLoadedForTenant, loadAccountPermissionsForTenant,
	// and loadAccountMetadata.
	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)

	// ARRANGE: Create tenant infrastructure (org + school with subdomain "t42")
	const tenantID int64 = 42
	testpkg.EnsureTestTenant(t, db, tenantID)

	// Register an account in tenant 42's context
	regCtx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("slug-valid")
	account, err := service.Register(regCtx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	// Ensure the account is mapped to tenant 42
	testpkg.MapAccountToTenant(t, db, account.ID, tenantID)

	// ACT: Login with explicit tenant slug "t42"
	accessToken, refreshToken, err := service.LoginWithAudit(
		context.Background(), email, testPassword, "", "", "t42",
	)

	// ASSERT: Should succeed and return valid tokens
	require.NoError(t, err, "LoginWithAudit should succeed with valid tenant slug")
	assert.NotEmpty(t, accessToken, "access token must not be empty")
	assert.NotEmpty(t, refreshToken, "refresh token must not be empty")
}

func TestLoginWithAudit_NonExistentTenantSlug(t *testing.T) {
	// LoginWithAudit with a slug that does not match any school should return
	// ErrTenantNotFound. Covers the resolveAccountTenantBySlug error path
	// where the school lookup fails.
	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)

	// ARRANGE: Create an account (tenant doesn't matter, we just need valid credentials)
	const tenantID int64 = 43
	testpkg.EnsureTestTenant(t, db, tenantID)

	regCtx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("slug-notfound")
	account, err := service.Register(regCtx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	// ACT: Login with a slug that matches no school
	_, _, err = service.LoginWithAudit(
		context.Background(), email, testPassword, "", "", "nonexistent-school-xyz",
	)

	// ASSERT: Should fail with ErrTenantNotFound
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrTenantNotFound),
		"expected ErrTenantNotFound, got: %v", err)
}

func TestLoginWithAudit_TenantSlugNoAccess(t *testing.T) {
	// LoginWithAudit where the tenant slug resolves to a valid school but the
	// account has no account_tenants mapping to it. Covers the
	// resolveAccountTenantBySlug "account does not have access" path.
	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)

	// ARRANGE: Create two tenants — account is in tenant 44, but will try to
	// login to tenant 45.
	const homeTenantID int64 = 44
	const otherTenantID int64 = 45
	testpkg.EnsureTestTenant(t, db, homeTenantID)
	testpkg.EnsureTestTenant(t, db, otherTenantID)

	regCtx := testpkg.TenantContext(homeTenantID)
	email, username := uniqueTestCredentials("slug-noaccess")
	account, err := service.Register(regCtx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	// Map account only to homeTenantID (44), NOT to otherTenantID (45)
	testpkg.MapAccountToTenant(t, db, account.ID, homeTenantID)

	// ACT: Login with slug "t45" — school exists but account has no access
	_, _, err = service.LoginWithAudit(
		context.Background(), email, testPassword, "", "", "t45",
	)

	// ASSERT: Should fail with ErrTenantAccessDenied
	require.Error(t, err)
	assert.True(t, errors.Is(err, auth.ErrTenantAccessDenied),
		"expected ErrTenantAccessDenied, got: %v", err)
}

func TestLoginWithAudit_EmptySlugDefaultResolution(t *testing.T) {
	// LoginWithAudit with an empty slug falls back to resolveAccountTenantDefault,
	// which picks the first active account_tenants mapping. Covers the default
	// resolution success path.
	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)

	// ARRANGE: Create tenant and register an account with a mapping
	const tenantID int64 = 46
	testpkg.EnsureTestTenant(t, db, tenantID)

	regCtx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("slug-default")
	account, err := service.Register(regCtx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	// Ensure the account has an active tenant mapping
	testpkg.MapAccountToTenant(t, db, account.ID, tenantID)

	// ACT: Login with empty slug — should resolve via default path
	accessToken, refreshToken, err := service.LoginWithAudit(
		context.Background(), email, testPassword, "", "", "",
	)

	// ASSERT: Should succeed
	require.NoError(t, err, "LoginWithAudit with empty slug should succeed when mapping exists")
	assert.NotEmpty(t, accessToken, "access token must not be empty")
	assert.NotEmpty(t, refreshToken, "refresh token must not be empty")
}

func TestLoginWithAudit_EmptySlugNoTenantMapping(t *testing.T) {
	// LoginWithAudit with an empty slug when the account has zero account_tenants
	// entries. resolveAccountTenantDefault returns ErrTenantNotFound because no
	// active tenant mappings exist. This confirms that accounts without any tenant
	// mapping cannot log in.
	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)

	// ARRANGE: Create a tenant for registration context, but do NOT map the account
	// to any tenant. Register creates the account but we will remove any auto-mapping.
	const tenantID int64 = 47
	testpkg.EnsureTestTenant(t, db, tenantID)

	regCtx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("slug-nomap")
	account, err := service.Register(regCtx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	// Remove any account_tenants mapping that Register may have created
	_, err = db.ExecContext(context.Background(),
		`DELETE FROM auth.account_tenants WHERE account_id = ?`, account.ID)
	require.NoError(t, err, "failed to remove account_tenants mapping")

	// ACT: Login with empty slug and no mappings
	_, _, err = service.LoginWithAudit(
		context.Background(), email, testPassword, "", "", "",
	)

	// ASSERT: Should fail — resolveAccountTenantDefault rejects accounts with no mappings
	require.Error(t, err, "LoginWithAudit should fail when account has no tenant mapping")
	assert.ErrorIs(t, err, auth.ErrTenantNotFound,
		"error should indicate no valid tenant was found")
}
