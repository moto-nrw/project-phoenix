package sessiontoken_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken/tokentest"
)

// The tests below pin the quota-identity contract from #2064: authenticated
// requests are bucketed by account ID + scope + tenant ID from verified
// claims, not by a per-token hash. Re-logins and token refreshes therefore
// share the account's budget instead of minting a fresh one.

func rateLimitTestAuth(t *testing.T) *sessiontoken.TokenAuth {
	t.Helper()
	tokenAuth, err := sessiontoken.NewTokenAuthWithDurations("rate-limit-test-secret-32-chars!!", 15*time.Minute, 24*time.Hour)
	require.NoError(t, err)
	return tokenAuth
}

func mintRateLimitJWT(t *testing.T, tokenAuth *sessiontoken.TokenAuth, claims sessiontoken.AppClaims) string {
	t.Helper()
	if len(claims.Roles) == 0 {
		claims.Roles = []string{"user"}
	}
	token, err := tokentest.CreateJWT(tokenAuth, claims)
	require.NoError(t, err)
	return token
}

// verifiedRateLimitKey runs the key behind the Verifier, as the router
// mounts the limiter.
func verifiedRateLimitKey(tokenAuth *sessiontoken.TokenAuth) func(*http.Request) string {
	return func(r *http.Request) string {
		var key string
		tokenAuth.Verifier()(http.HandlerFunc(func(_ http.ResponseWriter, verified *http.Request) {
			key = sessiontoken.RateLimitKey(verified)
		})).ServeHTTP(httptest.NewRecorder(), r)
		return key
	}
}

func rateLimitRequest(token string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/students", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

func TestIdentityRateLimitKey_TwoSessionsSameIdentityShareKey(t *testing.T) {
	t.Parallel()
	tokenAuth := rateLimitTestAuth(t)

	// Two distinct tokens (differing payloads) for the same account/tenant —
	// the re-login / second-browser-session scenario.
	sessionA := mintRateLimitJWT(t, tokenAuth, sessiontoken.AppClaims{
		ID: 42, Sub: "user@example.com", FirstName: "SessionA", TenantID: 7,
	})
	sessionB := mintRateLimitJWT(t, tokenAuth, sessiontoken.AppClaims{
		ID: 42, Sub: "user@example.com", FirstName: "SessionB", TenantID: 7,
	})
	require.NotEqual(t, sessionA, sessionB,
		"sessions must be distinct tokens for this test to prove anything")

	keyFunc := verifiedRateLimitKey(tokenAuth)
	assert.Equal(t, "acct:42::7", keyFunc(rateLimitRequest(sessionA)))
	assert.Equal(t, keyFunc(rateLimitRequest(sessionA)), keyFunc(rateLimitRequest(sessionB)))
}

func TestIdentityRateLimitKey_RefreshTokenSharesAccountBudget(t *testing.T) {
	t.Parallel()
	tokenAuth := rateLimitTestAuth(t)

	access := mintRateLimitJWT(t, tokenAuth, sessiontoken.AppClaims{
		ID: 42, Sub: "user@example.com", TenantID: 7,
	})
	refresh, err := tokentest.CreateRefreshJWT(tokenAuth, tokentest.RefreshClaims{
		ID: 42, Token: "opaque-refresh-token", TenantID: 7,
	})

	require.NoError(t, err)

	keyFunc := verifiedRateLimitKey(tokenAuth)
	assert.Equal(t, keyFunc(rateLimitRequest(access)), keyFunc(rateLimitRequest(refresh)),
		"a refresh token presented as bearer must land in the same account bucket")
}

func TestIdentityRateLimitKey_DifferentAccountsHaveDifferentKeys(t *testing.T) {
	t.Parallel()
	tokenAuth := rateLimitTestAuth(t)

	tokenA := mintRateLimitJWT(t, tokenAuth, sessiontoken.AppClaims{ID: 42, Sub: "user-a@example.com", TenantID: 7})
	tokenB := mintRateLimitJWT(t, tokenAuth, sessiontoken.AppClaims{ID: 43, Sub: "user-b@example.com", TenantID: 7})

	keyFunc := verifiedRateLimitKey(tokenAuth)
	assert.NotEqual(t, keyFunc(rateLimitRequest(tokenA)), keyFunc(rateLimitRequest(tokenB)))
}

func TestIdentityRateLimitKey_TenantSwitchGetsOwnBudget(t *testing.T) {
	t.Parallel()
	tokenAuth := rateLimitTestAuth(t)

	tenant7 := mintRateLimitJWT(t, tokenAuth, sessiontoken.AppClaims{ID: 42, Sub: "user@example.com", TenantID: 7})
	tenant8 := mintRateLimitJWT(t, tokenAuth, sessiontoken.AppClaims{ID: 42, Sub: "user@example.com", TenantID: 8})

	keyFunc := verifiedRateLimitKey(tokenAuth)
	assert.NotEqual(t, keyFunc(rateLimitRequest(tenant7)), keyFunc(rateLimitRequest(tenant8)),
		"a tenant switch mints a new tenant_id and gets its own budget")
}

func TestIdentityRateLimitKey_ScopeSeparatesPortals(t *testing.T) {
	t.Parallel()
	tokenAuth := rateLimitTestAuth(t)

	// Operator IDs and account IDs are different ID spaces — the same numeric
	// ID with different scopes must never share a bucket.
	tenantToken := mintRateLimitJWT(t, tokenAuth, sessiontoken.AppClaims{ID: 42, Sub: "staff@example.com", TenantID: 7})
	operatorToken := mintRateLimitJWT(t, tokenAuth, sessiontoken.AppClaims{ID: 42, Sub: "op@example.com", Scope: "platform"})
	parentToken := mintRateLimitJWT(t, tokenAuth, sessiontoken.AppClaims{ID: 42, Sub: "parent@example.com", Scope: "parent"})

	keyFunc := verifiedRateLimitKey(tokenAuth)
	keys := map[string]bool{
		keyFunc(rateLimitRequest(tenantToken)):   true,
		keyFunc(rateLimitRequest(operatorToken)): true,
		keyFunc(rateLimitRequest(parentToken)):   true,
	}
	assert.Len(t, keys, 3, "tenant, platform, and parent scopes must have distinct keys")
}

func TestIdentityRateLimitKey_KeyContainsNoTokenOrPII(t *testing.T) {
	t.Parallel()
	tokenAuth := rateLimitTestAuth(t)

	token := mintRateLimitJWT(t, tokenAuth, sessiontoken.AppClaims{
		ID: 42, Sub: "jane.doe@example.com", Username: "jane.doe@example.com",
		FirstName: "Jane", LastName: "Doe", TenantID: 7,
	})

	key := verifiedRateLimitKey(tokenAuth)(rateLimitRequest(token))
	assert.Equal(t, "acct:42::7", key)
	assert.NotContains(t, key, token)
	assert.NotContains(t, key, "jane.doe@example.com")
	assert.NotContains(t, key, "Jane")
	assert.NotContains(t, key, "Doe")
}

func TestIdentityRateLimitKey_InvalidTokenFallsBack(t *testing.T) {
	t.Parallel()
	tokenAuth := rateLimitTestAuth(t)

	assert.Empty(t, verifiedRateLimitKey(tokenAuth)(rateLimitRequest("not-a-real-token")))
}

func TestIdentityRateLimitKey_DeviceAPIKeyFallsBack(t *testing.T) {
	t.Parallel()
	// IoT devices send an opaque API key as bearer token. It is not a JWT and
	// cannot be verified here, so device requests must stay on IP limiting —
	// an unverified bearer value must never open its own bucket.
	tokenAuth := rateLimitTestAuth(t)

	assert.Empty(t, verifiedRateLimitKey(tokenAuth)(rateLimitRequest("phx_3f9c2d1a8b7e6f5a4d3c2b1a")))
}

func TestIdentityRateLimitKey_MissingHeaderFallsBack(t *testing.T) {
	t.Parallel()
	tokenAuth := rateLimitTestAuth(t)

	assert.Empty(t, verifiedRateLimitKey(tokenAuth)(rateLimitRequest("")))
}

func TestIdentityRateLimitKey_ExpiredTokenFallsBack(t *testing.T) {
	t.Parallel()
	tokenAuth := rateLimitTestAuth(t)

	_, token, err := tokenAuth.JwtAuth.Encode(map[string]any{
		"id":  42,
		"exp": time.Now().Add(-1 * time.Hour).Unix(),
		"iat": time.Now().Add(-2 * time.Hour).Unix(),
	})
	require.NoError(t, err)

	assert.Empty(t, verifiedRateLimitKey(tokenAuth)(rateLimitRequest(token)))
}

func TestIdentityRateLimitKey_TamperedTokenFallsBack(t *testing.T) {
	t.Parallel()
	tokenAuth := rateLimitTestAuth(t)
	otherAuth, err := sessiontoken.NewTokenAuthWithDurations("a-completely-different-32char-key!!", 15*time.Minute, time.Hour)
	require.NoError(t, err)

	forged := mintRateLimitJWT(t, otherAuth, sessiontoken.AppClaims{ID: 42, Sub: "user@example.com", TenantID: 7})

	assert.Empty(t, verifiedRateLimitKey(tokenAuth)(rateLimitRequest(forged)),
		"a token signed with the wrong secret must not produce an identity key")
}

func TestIdentityRateLimitKey_TokenWithoutAccountIDFallsBack(t *testing.T) {
	t.Parallel()
	// MFA challenge/enrollment tokens carry account_id + a pending flag but
	// no "id" claim — they must fall back to IP limiting.
	tokenAuth := rateLimitTestAuth(t)

	_, token, err := tokenAuth.JwtAuth.Encode(map[string]any{
		"account_id":  42,
		"mfa_pending": true,
		"exp":         time.Now().Add(5 * time.Minute).Unix(),
		"iat":         time.Now().Unix(),
	})
	require.NoError(t, err)

	assert.Empty(t, verifiedRateLimitKey(tokenAuth)(rateLimitRequest(token)))
}

func TestIdentityRateLimitKey_WithoutVerifierFallsBack(t *testing.T) {
	t.Parallel()
	// A request no Verifier checked keeps IP limiting: an unverified bearer
	// value never names a bucket.
	token := mintRateLimitJWT(t, rateLimitTestAuth(t), sessiontoken.AppClaims{ID: 42, Sub: "user@example.com", TenantID: 7})

	assert.Empty(t, sessiontoken.RateLimitKey(rateLimitRequest(token)))
}
