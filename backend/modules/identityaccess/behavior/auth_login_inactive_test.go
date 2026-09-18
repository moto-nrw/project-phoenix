package behavior_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// A disabled account is a fact about someone's account, so the login only
// tells it to a caller who presented that account's password (#3376).
// Before the reorder it answered "account is inactive" to anyone who typed
// the address, and the owner got the generic credentials error the frontend
// renders as "wrong password" — which is what sent Burbach's headmaster
// through password resets that could not help.
func TestLoginReportsDisabledAccountOnlyAfterTheCorrectPassword(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)
	tenantID := testpkg.Tenant(t)

	email, username := uniqueTestCredentials("login-disabled")
	account, err := service.Register(testpkg.TenantContext(tenantID), email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.MapAccountToTenant(t, db, account.ID, tenantID)
	setAccountActive(t, db, account.ID, false)

	t.Run("wrong password stays a credentials error", func(t *testing.T) {
		_, _, loginErr := service.LoginWithAudit(context.Background(), email, "Wr0ng!Password", "", "", "")

		require.Error(t, loginErr)
		require.ErrorIs(t, loginErr, identityaccess.ErrInvalidCredentials)
		require.NotErrorIs(t, loginErr, identityaccess.ErrAccountInactive,
			"an unauthenticated caller must not learn that the address belongs to a disabled account")
	})

	t.Run("correct password names the real reason", func(t *testing.T) {
		_, _, loginErr := service.LoginWithAudit(context.Background(), email, testPassword, "", "", "")

		require.Error(t, loginErr)
		require.ErrorIs(t, loginErr, identityaccess.ErrAccountInactive)
	})

	t.Run("re-enabled account signs in again", func(t *testing.T) {
		setAccountActive(t, db, account.ID, true)

		accessToken, refreshToken, loginErr := service.LoginWithAudit(context.Background(), email, testPassword, "", "", "")

		require.NoError(t, loginErr)
		require.NotEmpty(t, accessToken)
		require.NotEmpty(t, refreshToken)
	})
}
