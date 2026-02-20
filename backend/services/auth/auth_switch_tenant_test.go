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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupAuthService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("fails for non-existent account", func(t *testing.T) {
		// ACT
		accessToken, refreshToken, err := service.SwitchTenant(ctx, 999999, "t1")

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
		account, err := service.Register(ctx, email, username, testPassword, nil)
		require.NoError(t, err)
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		// Deactivate the account
		_, dbErr := db.Exec("UPDATE auth.accounts SET active = false WHERE id = ?", account.ID)
		require.NoError(t, dbErr)

		// ACT
		accessToken, refreshToken, err := service.SwitchTenant(ctx, account.ID, "t1")

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
		account, err := service.Register(ctx, email, username, testPassword, nil)
		require.NoError(t, err)
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		// ACT: Use a non-existent tenant slug
		accessToken, refreshToken, err := service.SwitchTenant(ctx, account.ID, "nonexistent-tenant")

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
		account, err := service.Register(ctx, email, username, testPassword, nil)
		require.NoError(t, err)
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		// Ensure tenant 3 exists but DON'T map the account to it
		testpkg.EnsureTestTenant(t, db, 3)

		// ACT: Try switching to tenant 3 (no account_tenants mapping)
		accessToken, refreshToken, err := service.SwitchTenant(ctx, account.ID, "t3")

		// ASSERT
		require.Error(t, err)
		assert.Empty(t, accessToken)
		assert.Empty(t, refreshToken)
	})
}
