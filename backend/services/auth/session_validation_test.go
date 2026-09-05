package auth_test

import (
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	authService "github.com/moto-nrw/project-phoenix/services/auth"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestValidateSessionTokens(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	// NewService retains the aggregate type; validation uses only these five repositories.
	persistence := repositories.NewSessionValidationPersistence(db)
	repos := &repositories.Factory{
		Account:              persistence.Account,
		AccountTenant:        persistence.AccountTenant,
		Token:                persistence.Token,
		Operator:             persistence.Operator,
		OperatorRefreshToken: persistence.OperatorRefreshToken,
	}
	signer, err := jwt.NewTokenAuthWithDurations("session-validation-test-signing-key", 15*time.Minute, time.Hour)
	require.NoError(t, err)
	cfg, err := authService.NewServiceConfig(nil, email.Email{}, "http://localhost:3000", time.Hour)
	require.NoError(t, err)
	cfg.TokenAuth = signer
	service, err := authService.NewService(repos, cfg, db, slog.Default())
	require.NoError(t, err)
	testpkg.SetTenantRuntime(t, service, db)
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
				row := &platformModels.OperatorRefreshToken{OperatorID: accountID, Token: handle, Expiry: time.Now().Add(time.Hour), FamilyID: family}
				require.NoError(t, repos.OperatorRefreshToken.Create(ctx, row))
				tokenID = row.ID
			} else {
				row := &authModels.Token{AccountID: accountID, Token: handle, Expiry: time.Now().Add(time.Hour), FamilyID: family, PortalScope: portal}
				row.TenantID = testpkg.Tenant(t)
				require.NoError(t, repos.Token.Create(ctx, row))
				tokenID = row.ID
			}
			accessClaims := jwt.AppClaims{ID: int(accountID), Sub: account.Email, Roles: []string{"user"}, Scope: scope, TenantID: tenantID, FamilyID: family}
			refreshClaims := jwt.RefreshClaims{ID: int(accountID), Token: handle, Scope: scope, TenantID: tenantID}
			access, refresh, err := signer.GenTokenPair(accessClaims, refreshClaims)
			require.NoError(t, err)
			verified, err := service.ValidateSessionTokens(ctx, access, refresh, portal)
			require.NoError(t, err)
			require.Equal(t, int(accountID), verified.ID)
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
			otherSigner, err := jwt.NewTokenAuthWithDurations("different-test-signing-key-32-characters", time.Minute, time.Hour)
			require.NoError(t, err)
			forged, _, err := otherSigner.GenTokenPair(accessClaims, refreshClaims)
			require.NoError(t, err)
			_, err = service.ValidateSessionTokens(ctx, forged, refresh, portal)
			require.Error(t, err)
			for _, flag := range []string{"mfa_pending", "mfa_enrollment_pending", "missing_expiry"} {
				raw, err := jwt.ParseStructToMap(accessClaims)
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
			rawClaims, err := jwt.ParseStructToMap(accessClaims)
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
				require.NoError(t, repos.OperatorRefreshToken.Delete(ctx, tokenID))
			} else {
				require.NoError(t, repos.Token.Delete(ctx, tokenID))
			}
			_, err = service.ValidateSessionTokens(ctx, access, refresh, portal)
			require.Error(t, err)
		})
	}
}
