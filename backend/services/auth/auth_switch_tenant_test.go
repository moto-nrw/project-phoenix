package auth_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/services/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthService_SwitchTenant(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("fails for non-existent account", func(t *testing.T) {
		// ACT
		accessToken, refreshToken, err := service.SwitchTenant(ctx, 999999, "t1", "")

		// ASSERT
		require.Error(t, err)
		assert.Empty(t, accessToken)
		assert.Empty(t, refreshToken)

		var authErr *auth.AuthError
		if assert.ErrorAs(t, err, &authErr) {
			assert.ErrorIs(t, authErr.Err, auth.ErrAccountNotFound)
		}
	})

	t.Run("fails for inactive account", func(t *testing.T) {
		// ARRANGE: Create account and deactivate it
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		email := fmt.Sprintf("inactive-%s@test.local", uniqueID)
		username := fmt.Sprintf("inactive-%s", uniqueID)
		account, err := service.Register(ctx, email, username, testPassword, nil, 0)
		require.NoError(t, err)

		// Deactivate the account
		_, dbErr := db.Exec("UPDATE auth.accounts SET active = false WHERE id = ?", account.ID)
		require.NoError(t, dbErr)

		// ACT
		accessToken, refreshToken, err := service.SwitchTenant(ctx, account.ID, "t1", "")

		// ASSERT
		require.Error(t, err)
		assert.Empty(t, accessToken)
		assert.Empty(t, refreshToken)

		var authErr *auth.AuthError
		if assert.ErrorAs(t, err, &authErr) {
			assert.ErrorIs(t, authErr.Err, auth.ErrAccountInactive)
		}
	})

	t.Run("fails for non-existent tenant slug", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		email := fmt.Sprintf("badslug-%s@test.local", uniqueID)
		username := fmt.Sprintf("badslug-%s", uniqueID)
		account, err := service.Register(ctx, email, username, testPassword, nil, 0)
		require.NoError(t, err)

		// ACT: Use a non-existent tenant slug
		accessToken, refreshToken, err := service.SwitchTenant(ctx, account.ID, "nonexistent-tenant", "")

		// ASSERT
		require.Error(t, err)
		assert.Empty(t, accessToken)
		assert.Empty(t, refreshToken)
	})

	t.Run("fails when account has no access to target tenant", func(t *testing.T) {
		// ARRANGE: Create account mapped only to tenant 1
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		email := fmt.Sprintf("noaccess-%s@test.local", uniqueID)
		username := fmt.Sprintf("noaccess-%s", uniqueID)
		account, err := service.Register(ctx, email, username, testPassword, nil, 0)
		require.NoError(t, err)

		// Ensure the target exists but DON'T map the account to it.
		targetTenantID := testpkg.UniqueTestTenantID(t)
		testpkg.EnsureTestTenant(t, db, targetTenantID)

		// ACT: Try switching to the target (no account_tenants mapping)
		accessToken, refreshToken, err := service.SwitchTenant(ctx, account.ID, fmt.Sprintf("t%d", targetTenantID), "")

		// ASSERT
		require.Error(t, err)
		assert.Empty(t, accessToken)
		assert.Empty(t, refreshToken)
	})

	t.Run("switches tenant successfully", func(t *testing.T) {
		// ARRANGE: Create account mapped to tenant 1, then switch to tenant 2
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		email := fmt.Sprintf("switch-%s@test.local", uniqueID)
		username := fmt.Sprintf("switch-%s", uniqueID)
		account, err := service.Register(ctx, email, username, testPassword, nil, 0)
		require.NoError(t, err)

		targetTenantID := testpkg.UniqueTestTenantID(t)
		testpkg.EnsureTestTenant(t, db, targetTenantID)
		testpkg.MapAccountToTenant(t, db, account.ID, targetTenantID)

		// ACT
		accessToken, refreshToken, err := service.SwitchTenant(ctx, account.ID, fmt.Sprintf("t%d", targetTenantID), "")

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, accessToken)
		assert.NotEmpty(t, refreshToken)
	})
}

// TestLogoutInvalidatesTokensBeforeTenantSwitch reproduces the scenario from issue #1067:
//
//  1. User A logs in on tenant 1, switches to tenant 2
//  2. User A logs out
//  3. User B logs in on tenant 1, switches to tenant 2
//  4. User B should see their own identity, NOT User A's
//
// The root cause was that logout never actually deleted backend tokens (frontend
// sent access token instead of refresh token). This test verifies that after a
// proper logout, the old user's refresh tokens are invalidated and a different
// user can switch tenants cleanly.
func TestLogoutInvalidatesTokensBeforeTenantSwitch(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	targetTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, targetTenantID)

	// ARRANGE: Create User A (admin) mapped to both tenants
	emailA, usernameA := uniqueTestCredentials("userA-1067")
	accountA, err := service.Register(ctx, emailA, usernameA, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.MapAccountToTenant(t, db, accountA.ID, targetTenantID)

	// ARRANGE: Create User B (betreuer) mapped to both tenants
	emailB, usernameB := uniqueTestCredentials("userB-1067")
	accountB, err := service.Register(ctx, emailB, usernameB, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.MapAccountToTenant(t, db, accountB.ID, targetTenantID)

	// Step 1: User A logs in on tenant 1
	_, refreshTokenA, err := service.Login(ctx, emailA, testPassword)
	require.NoError(t, err, "User A login should succeed")

	// Step 2: User A switches to tenant 2
	_, _, err = service.SwitchTenant(ctx, accountA.ID, fmt.Sprintf("t%d", targetTenantID), "")
	require.NoError(t, err, "User A switch to tenant 2 should succeed")

	// Step 3: User A logs out — this should invalidate User A's tokens
	err = service.LogoutWithAudit(ctx, refreshTokenA, "", "")
	require.NoError(t, err, "User A logout should succeed")

	// Verify: User A's refresh token is now invalid
	_, _, err = service.RefreshToken(ctx, refreshTokenA)
	require.Error(t, err, "User A's refresh token should be invalidated after logout")

	// Step 4: User B logs in on tenant 1
	_, _, err = service.Login(ctx, emailB, testPassword)
	require.NoError(t, err, "User B login should succeed")

	// Step 5: User B switches to tenant 2 — should get User B's tokens, NOT User A's
	accessTokenB, _, err := service.SwitchTenant(ctx, accountB.ID, fmt.Sprintf("t%d", targetTenantID), "")
	require.NoError(t, err, "User B switch to tenant 2 should succeed")
	assert.NotEmpty(t, accessTokenB, "User B should receive a valid access token")
}

// TestLogoutInvalidatesRefreshToken verifies that after logout, the refresh
// token used to authenticate is invalidated and cannot be reused.
func TestLogoutInvalidatesRefreshToken(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	// ARRANGE
	email, username := uniqueTestCredentials("logout-invalidate")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

	_, refreshToken, err := service.Login(ctx, email, testPassword)
	require.NoError(t, err)

	// Refresh should work before logout
	_, _, err = service.RefreshToken(ctx, refreshToken)
	require.NoError(t, err, "refresh should succeed before logout")

	// ACT: Logout
	err = service.LogoutWithAudit(ctx, refreshToken, "", "")
	require.NoError(t, err)

	// ASSERT: Refresh must fail after logout
	_, _, err = service.RefreshToken(ctx, refreshToken)
	require.Error(t, err, "refresh should fail after logout — token was deleted")
}
