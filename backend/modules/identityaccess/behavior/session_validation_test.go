package behavior_test

import (
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// TestValidateSessionTokens drives the retained AuthService's session
// validation, which delegates to Identity & Access (#3251). Sessions are
// persisted through the owner's public capability; the signer shares the
// factory's JWT configuration so the minted pairs verify.
func TestValidateSessionTokens(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	config := authTestFactoryConfig(false)
	identityModule, err := api.NewTestIdentityAccessWithConfig(testpkg.NewDatabase(t, db), slog.Default(), config)
	require.NoError(t, err)
	service := identityaccess.AccountAuthentication(identityModule)
	require.NotNil(t, service, "the composed module validates session hand-offs")
	signer, err := sessiontoken.NewTokenAuthWithDurations(config.JWTSecret, config.JWTExpiry, config.JWTRefreshExpiry)
	require.NoError(t, err)
	sessions, err := identityaccessCompose.New(identityaccessCompose.Dependencies{DB: db, TenantRuntime: testpkg.TenantRuntime(t, db), Observe: func(identityaccessCompose.Observation) {}})
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	for _, portal := range []string{"tenant", "parent", "school", "platform"} {
		t.Run(portal, func(t *testing.T) {
			account := testpkg.CreateTestAccount(t, db, "session-validation")
			testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
			scope := portal
			tenantID := testpkg.Tenant(t)
			accountID := account.ID
			if portal == "tenant" {
				scope = ""
			}
			if portal == "parent" || portal == "platform" {
				tenantID = 0
			}
			handle := fmt.Sprintf("session-handle-%d", time.Now().UnixNano())
			var tokenID int64
			family := fmt.Sprintf("session-family-%d", time.Now().UnixNano())
			if portal == "platform" {
				operator := testpkg.CreateTestOperator(t, db)
				accountID = operator.ID
				stored, err := sessions.CreateOperatorSession(ctx, identityaccess.OperatorSession{
					OperatorID: accountID, Token: handle, Expiry: time.Now().Add(time.Hour), FamilyID: family,
				})
				require.NoError(t, err)
				tokenID = stored.ID
			} else {
				stored, err := sessions.CreateAccountSession(ctx, identityaccess.AccountSession{
					AccountID: accountID, TenantID: testpkg.Tenant(t), Token: handle, Expiry: time.Now().Add(time.Hour), FamilyID: family, PortalScope: portal,
				})
				require.NoError(t, err)
				tokenID = stored.ID
			}
			accessClaims := sessiontoken.AppClaims{ID: int(accountID), Sub: account.Email, Roles: []string{"user"}, Scope: scope, TenantID: tenantID, FamilyID: family}
			refreshClaims := sessiontoken.RefreshClaims{ID: int(accountID), Token: handle, Scope: scope, TenantID: tenantID}
			access, refresh, err := signer.GenTokenPair(accessClaims, refreshClaims)
			require.NoError(t, err)
			verified, err := service.ValidateSessionTokens(ctx, access, refresh, portal)
			require.NoError(t, err)
			require.Equal(t, accountID, verified.AccountID)
			// Validation is non-consuming: the same pair remains valid.
			_, err = service.ValidateSessionTokens(ctx, access, refresh, portal)
			require.NoError(t, err)
			for _, wrongPortal := range []string{"tenant", "parent", "school", "platform"} {
				if wrongPortal == portal {
					continue
				}
				_, err = service.ValidateSessionTokens(ctx, access, refresh, wrongPortal)
				require.Error(t, err)
			}
			_, err = service.ValidateSessionTokens(ctx, access, "arbitrary", portal)
			require.Error(t, err)
			_, err = service.ValidateSessionTokens(ctx, access+"tampered", refresh, portal)
			require.Error(t, err)
			otherSigner, err := sessiontoken.NewTokenAuthWithDurations("different-test-signing-key-32-characters", time.Minute, time.Hour)
			require.NoError(t, err)
			forged, _, err := otherSigner.GenTokenPair(accessClaims, refreshClaims)
			require.NoError(t, err)
			_, err = service.ValidateSessionTokens(ctx, forged, refresh, portal)
			require.Error(t, err)
			for _, flag := range []string{"mfa_pending", "mfa_enrollment_pending", "missing_expiry"} {
				raw, err := sessiontoken.ParseStructToMap(accessClaims)
				require.NoError(t, err)
				raw["exp"] = time.Now().Add(time.Minute).Unix()
				if flag == "missing_expiry" {
					delete(raw, "exp")
				} else {
					raw[flag] = true
				}
				_, unfinished, err := signer.JwtAuth.Encode(raw)
				require.NoError(t, err)
				_, err = service.ValidateSessionTokens(ctx, unfinished, refresh, portal)
				require.Error(t, err, flag)
			}
			accessClaims.FamilyID = "another-family"
			wrongFamily, _, err := signer.GenTokenPair(accessClaims, refreshClaims)
			require.NoError(t, err)
			_, err = service.ValidateSessionTokens(ctx, wrongFamily, refresh, portal)
			require.Error(t, err)
			accessClaims.FamilyID = family
			accessClaims.ExpiresAt = time.Now().Add(-time.Minute).Unix()
			rawClaims, err := sessiontoken.ParseStructToMap(accessClaims)
			require.NoError(t, err)
			_, expired, err := signer.JwtAuth.Encode(rawClaims)
			require.NoError(t, err)
			_, err = service.ValidateSessionTokens(ctx, expired, refresh, portal)
			require.Error(t, err)
			refreshClaims.ExpiresAt = time.Now().Add(-time.Minute).Unix()
			expiredRefresh, err := signer.CreateRefreshJWT(refreshClaims)
			require.NoError(t, err)
			_, err = service.ValidateSessionTokens(ctx, access, expiredRefresh, portal)
			require.Error(t, err)
			refreshClaims.ExpiresAt = 0
			refreshClaims.Token = "signed-but-unpersisted"
			unknownRefresh, err := signer.CreateRefreshJWT(refreshClaims)
			require.NoError(t, err)
			_, err = service.ValidateSessionTokens(ctx, access, unknownRefresh, portal)
			require.Error(t, err)
			if portal == "platform" {
				require.NoError(t, sessions.DeleteOperatorSession(ctx, tokenID))
			} else {
				require.NoError(t, sessions.DeleteAccountSession(ctx, tokenID))
			}
			_, err = service.ValidateSessionTokens(ctx, access, refresh, portal)
			require.Error(t, err)
		})
	}
}
