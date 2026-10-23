package account_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	authAPI "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/account"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestTenantSwitchWithoutMFAProofIsUnauthorized(t *testing.T) {
	t.Parallel()
	tc, router := setupProtectedRouter(t)
	password := testpkg.NewTestPassword()
	account := testpkg.CreateTestAccountWithPassword(t, tc.db, "switch-mfa@example.test", password)
	testpkg.EnsureAccountTenant(t, tc.db, account.ID, testpkg.Tenant(t))
	access, _, err := tc.resource.Sessions.LoginWithAudit(context.Background(), account.Email, password, "", "", "")
	require.NoError(t, err)
	target, slug := testpkg.CreateTestTenant(t, tc.db)
	testpkg.EnsureAccountTenant(t, tc.db, account.ID, target)
	testpkg.GrantTestPermission(t, tc.db, target, account.ID, "student_documents:health")
	req := testutil.NewJSONRequest(t, http.MethodPost, "/auth/switch-tenant", map[string]string{"tenant_slug": slug})
	req.Header.Set("Authorization", "Bearer "+access)
	response := testutil.ExecuteRequest(router, req)
	require.Equal(t, http.StatusUnauthorized, response.Code)
	var problem struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &problem))
	require.Equal(t, "general.permission", problem.Code)
}

func TestTenantSwitchPreservesVerifiedMFAAfterRotation(t *testing.T) {
	t.Parallel()
	tc, router := setupProtectedRouter(t)
	account := testpkg.CreateTestAccount(t, tc.db, "verified-switch-mfa@example.test")
	source := testpkg.Tenant(t)
	testpkg.EnsureAccountTenant(t, tc.db, account.ID, source)
	testpkg.GrantTestPermission(t, tc.db, source, account.ID, "student_documents:health")
	target, slug := testpkg.CreateTestTenant(t, tc.db)
	testpkg.EnsureAccountTenant(t, tc.db, account.ID, target)
	testpkg.GrantTestPermission(t, tc.db, target, account.ID, "staff:financial")
	// The same capability completes a successful email/passkey MFA exchange.
	_, refresh, err := tc.resource.Sessions.IssueTokensForAuthenticatedAccount(context.Background(), account.ID, source, "", "")
	require.NoError(t, err)
	access, _, err := tc.resource.Sessions.RefreshTokenWithAudit(context.Background(), refresh, "", "", "")
	require.NoError(t, err)
	req := testutil.NewJSONRequest(t, http.MethodPost, "/auth/switch-tenant", map[string]string{"tenant_slug": slug})
	req.Header.Set("Authorization", "Bearer "+access)
	response := testutil.ExecuteRequest(router, req)
	require.Equal(t, http.StatusOK, response.Code)
	var tokens authAPI.TokenResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &tokens))
	require.NotEmpty(t, tokens.AccessToken)
	require.NotEmpty(t, tokens.RefreshToken)
	claims, err := tc.resource.Sessions.ValidateSessionTokens(context.Background(), tokens.AccessToken, tokens.RefreshToken, "tenant")
	require.NoError(t, err)
	require.Equal(t, target, claims.TenantID)
	_, _, err = tc.resource.Sessions.RefreshTokenWithAudit(context.Background(), tokens.RefreshToken, "", "", "")
	require.NoError(t, err, "the switched session must retain verified MFA through refresh")
}

func TestTenantSwitchCannotRestoreInvalidatedMFAProof(t *testing.T) {
	t.Parallel()
	for _, invalidation := range []string{"mfa-reset", "logout", "expired"} {
		t.Run(invalidation, func(t *testing.T) {
			testpkg.OwnTenant(t)
			tc, router := setupProtectedRouter(t)
			account := testpkg.CreateTestAccount(t, tc.db, "invalidated-switch@example.test")
			source := testpkg.Tenant(t)
			testpkg.EnsureAccountTenant(t, tc.db, account.ID, source)
			target, slug := testpkg.CreateTestTenant(t, tc.db)
			testpkg.EnsureAccountTenant(t, tc.db, account.ID, target)
			testpkg.GrantTestPermission(t, tc.db, target, account.ID, "student_documents:health")
			access, refresh, err := tc.resource.Sessions.IssueTokensForAuthenticatedAccount(context.Background(), account.ID, source, "", "")
			require.NoError(t, err)
			switch invalidation {
			case "mfa-reset":
				module := setupAuthModuleOn(t, tc.db)
				require.NoError(t, module.MFA.DisableMFA(testpkg.Ctx(t), account.ID))
			case "logout":
				require.NoError(t, tc.resource.Sessions.LogoutWithAudit(context.Background(), refresh, "", ""))
			case "expired":
				_, err := tc.db.ExecContext(context.Background(), `UPDATE auth.tokens SET expiry = NOW() - INTERVAL '1 second' WHERE account_id = ?`, account.ID)
				require.NoError(t, err)
			}
			// The access JWT is still signed and unexpired, but no live proof remains.
			req := testutil.NewJSONRequest(t, http.MethodPost, "/auth/switch-tenant", map[string]string{"tenant_slug": slug})
			req.Header.Set("Authorization", "Bearer "+access)
			response := testutil.ExecuteRequest(router, req)
			require.Equal(t, http.StatusUnauthorized, response.Code)
			var problem struct {
				Code string `json:"code"`
			}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &problem))
			require.Equal(t, "general.permission", problem.Code)
		})
	}
}
