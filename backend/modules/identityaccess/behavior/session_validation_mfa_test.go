package behavior_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// A session without second-factor proof that its refresh refuses is refused
// by the validation too, with the same outcome: the floor is one of the
// facts a renewal decides.
func TestValidateSessionTokens_AppliesTheRefreshMFAFloor(t *testing.T) {
	t.Parallel()

	t.Run("caterer switched its factor on", func(t *testing.T) {
		t.Parallel()
		sc := newCatererMFAScenario(t)
		before, err := sc.login(t, "")
		require.NoError(t, err)
		sc.switchOn(t)

		requireValidationEndsLikeTheRefresh(t, sc.module, before.AccessToken, before.RefreshToken, "caterer")
	})

	t.Run("school requires the factor of all staff", func(t *testing.T) {
		t.Parallel()
		db := testpkg.SetupTestDB(t)
		module := newMFATestModule(t, db, withDeliveringMailer(nil)...)
		service := newFixtureAuthService(t, db, module.Auth)
		tenantID, _ := newSchoolTenant(t, db)
		email, _ := createLehrkraftAccount(t, db, service, "school-validate-mfa", tenantID)
		login, err := service.LoginSchoolWithMFAGate(context.Background(), email, testPassword, "", "", "")
		require.NoError(t, err)
		require.Equal(t, identityaccess.LoginStatusAuthenticated, login.Status)
		writeTenantSettingOverride(t, db, tenantID, settingKeyMFAMode, mfaModeRequiredAll)

		requireValidationEndsLikeTheRefresh(t, module.Auth, login.AccessToken, login.RefreshToken, "school")
	})

	t.Run("tenant staff gained a privileged role", func(t *testing.T) {
		t.Parallel()
		db := testpkg.SetupTestDB(t)
		module := newMFATestModule(t, db, withDeliveringMailer(nil)...)
		service := newFixtureAuthService(t, db, module.Auth)
		tenantID, _ := newSchoolTenant(t, db)
		email, accountID := newSchoolAccount(t, db, service, "tenant-validate-mfa", tenantID)
		assignSeededRole(t, db, accountID, tenantID, "user")
		login, err := module.Auth.LoginWithMFAGate(context.Background(), email, testPassword, "", "", "", "")
		require.NoError(t, err)
		require.Equal(t, identityaccess.LoginStatusAuthenticated, login.Status)
		assignSeededRole(t, db, accountID, tenantID, "admin")

		requireValidationEndsLikeTheRefresh(t, module.Auth, login.AccessToken, login.RefreshToken, "tenant")
	})
}

// requireValidationEndsLikeTheRefresh validates the session with and
// without its refresh token, then refreshes it: all three end it.
func requireValidationEndsLikeTheRefresh(t *testing.T, module identityaccess.Owner, access, refresh, portal string) {
	t.Helper()
	ctx := context.Background()
	for _, presented := range []string{"", refresh} {
		_, err := module.ValidateSessionTokens(ctx, access, presented, portal)
		require.ErrorIs(t, err, identityaccess.ErrInvalidToken, "with refresh token: %t", presented != "")
		assert.Equal(t, identityaccess.SessionEnded, identityaccess.SessionOutcomeOf(err))
	}
	_, _, err := module.RefreshTokenWithAudit(ctx, refresh, "", "", "")
	assert.Equal(t, identityaccess.SessionEnded, identityaccess.SessionOutcomeOf(err), "the refresh ends it too: %v", err)
}
