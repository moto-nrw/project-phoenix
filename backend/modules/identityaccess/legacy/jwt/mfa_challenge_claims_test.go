package jwt

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/ptrtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMFAChallengeClaims_TenantRoundTrip(t *testing.T) {
	t.Parallel()

	ta, err := newTestTokenAuth(t, "test-secret-must-be-at-least-32-characters-long")
	require.NoError(t, err)

	in := MFAChallengeClaims{
		AccountID: 4242,
		Scope:     MFAChallengeScopeTenant,
		TenantID:  4242,
	}

	tokenString, err := ta.CreateMFAChallengeJWT(in, 5*time.Minute)
	require.NoError(t, err)
	require.NotEmpty(t, tokenString)

	jwtToken, err := ta.JwtAuth.Decode(tokenString)
	require.NoError(t, err)

	raw := make(map[string]any)
	for _, k := range jwtToken.Keys() {
		var v any
		if err := jwtToken.Get(k, &v); err == nil {
			raw[k] = v
		}
	}

	var out MFAChallengeClaims
	require.NoError(t, out.ParseClaims(raw))
	assert.Equal(t, int64(4242), out.AccountID)
	assert.Equal(t, MFAChallengeScopeTenant, out.Scope)
	assert.Equal(t, int64(4242), out.TenantID)
	assert.True(t, out.MFAPending)
	assert.Greater(t, out.ExpiresAt, time.Now().Unix())
}

func TestMFAChallengeClaims_PlatformRoundTrip(t *testing.T) {
	t.Parallel()

	ta, err := newTestTokenAuth(t, "test-secret-must-be-at-least-32-characters-long")
	require.NoError(t, err)

	in := MFAChallengeClaims{
		AccountID: 99,
		Scope:     MFAChallengeScopePlatform,
	}

	tokenString, err := ta.CreateMFAChallengeJWT(in, 5*time.Minute)
	require.NoError(t, err)

	jwtToken, err := ta.JwtAuth.Decode(tokenString)
	require.NoError(t, err)
	raw := make(map[string]any)
	for _, k := range jwtToken.Keys() {
		var v any
		if err := jwtToken.Get(k, &v); err == nil {
			raw[k] = v
		}
	}

	var out MFAChallengeClaims
	require.NoError(t, out.ParseClaims(raw))
	assert.Equal(t, MFAChallengeScopePlatform, out.Scope)
	assert.Equal(t, int64(0), out.TenantID, "platform tokens should not carry tenant_id")
}

func TestMFAChallengeClaims_RejectsMissingAccountID(t *testing.T) {
	t.Parallel()

	var c MFAChallengeClaims
	err := c.ParseClaims(map[string]any{
		"mfa_pending": true,
		"scope":       MFAChallengeScopeTenant,
	})
	assert.Error(t, err)
}

func TestMFAChallengeClaims_RejectsBadScope(t *testing.T) {
	t.Parallel()

	var c MFAChallengeClaims
	err := c.ParseClaims(map[string]any{
		"account_id":  float64(1),
		"mfa_pending": true,
		"scope":       "bogus",
	})
	assert.Error(t, err)
}

func TestMFAChallengeClaims_RejectsNonPendingToken(t *testing.T) {
	t.Parallel()

	var c MFAChallengeClaims
	err := c.ParseClaims(map[string]any{
		"account_id":  float64(1),
		"scope":       MFAChallengeScopeTenant,
		"mfa_pending": false,
	})
	assert.Error(t, err)
}

// ParseMFAChallengeJWT is the end-to-end wrapper both the tenant and the
// operator MFA verify handlers funnel through, but its previous coverage was
// 0 % because the round-trip tests above stopped at ParseClaims. These cases
// exercise the JWT-decode + ParseClaims composition through the public entry
// point so refactors that touch the decode loop or the expiry check get
// caught.

func TestParseMFAChallengeJWT_TenantRoundTripPopulatesClaims(t *testing.T) {
	t.Parallel()

	ta, err := newTestTokenAuth(t, "test-secret-must-be-at-least-32-characters-long")
	require.NoError(t, err)

	tokenString, err := ta.CreateMFAChallengeJWT(MFAChallengeClaims{
		AccountID: 4242,
		Scope:     MFAChallengeScopeTenant,
		TenantID:  70010001,
	}, 5*time.Minute)
	require.NoError(t, err)

	claims, err := ta.ParseMFAChallengeJWT(tokenString)
	require.NoError(t, err)
	require.NotNil(t, claims)
	assert.Equal(t, int64(4242), claims.AccountID)
	assert.Equal(t, MFAChallengeScopeTenant, claims.Scope)
	assert.Equal(t, int64(70010001), claims.TenantID)
	assert.True(t, claims.MFAPending, "MFAPending must round-trip through the public entry point")
}

func TestParseMFAChallengeJWT_PlatformRoundTripOmitsTenant(t *testing.T) {
	t.Parallel()

	ta, err := newTestTokenAuth(t, "test-secret-must-be-at-least-32-characters-long")
	require.NoError(t, err)

	tokenString, err := ta.CreateMFAChallengeJWT(MFAChallengeClaims{
		AccountID: 99,
		Scope:     MFAChallengeScopePlatform,
	}, 5*time.Minute)
	require.NoError(t, err)

	claims, err := ta.ParseMFAChallengeJWT(tokenString)
	require.NoError(t, err)
	assert.Equal(t, MFAChallengeScopePlatform, claims.Scope)
	assert.Equal(t, int64(0), claims.TenantID,
		"platform tokens must not surface a tenant_id even after round-trip")
}

func TestParseMFAChallengeJWT_RejectsGarbageToken(t *testing.T) {
	t.Parallel()

	ta, err := newTestTokenAuth(t, "test-secret-must-be-at-least-32-characters-long")
	require.NoError(t, err)

	claims, err := ta.ParseMFAChallengeJWT("not-a-jwt-at-all")
	require.Error(t, err)
	assert.Nil(t, claims)
}

func TestParseMFAChallengeJWT_RejectsTokenSignedWithDifferentSecret(t *testing.T) {
	t.Parallel()

	tenantID := ptrtest.NewTenantID()
	issuer, err := newTestTokenAuth(t, "issuer-secret-must-be-at-least-32-characters-long")
	require.NoError(t, err)
	verifier, err := newTestTokenAuth(t, "verifier-secret-must-be-at-least-32-characters-long")
	require.NoError(t, err)

	tokenString, err := issuer.CreateMFAChallengeJWT(MFAChallengeClaims{
		AccountID: 1,
		Scope:     MFAChallengeScopeTenant,
		TenantID:  tenantID,
	}, 5*time.Minute)
	require.NoError(t, err)

	claims, err := verifier.ParseMFAChallengeJWT(tokenString)
	require.Error(t, err, "signature mismatch must surface as a decode error")
	assert.Nil(t, claims)
}

func TestParseMFAChallengeJWT_RejectsExpiredToken(t *testing.T) {
	t.Parallel()

	tenantID := ptrtest.NewTenantID()
	ta, err := newTestTokenAuth(t, "test-secret-must-be-at-least-32-characters-long")
	require.NoError(t, err)

	// Issue with a TTL that's already in the past so the exp-check branch fires.
	tokenString, err := ta.CreateMFAChallengeJWT(MFAChallengeClaims{
		AccountID: 1,
		Scope:     MFAChallengeScopeTenant,
		TenantID:  tenantID,
	}, -1*time.Second)
	require.NoError(t, err)

	claims, err := ta.ParseMFAChallengeJWT(tokenString)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expired",
		"expired tokens must be rejected with an explicit message")
	assert.Nil(t, claims)
}

func TestParseMFAChallengeJWT_RejectsNonChallengeToken(t *testing.T) {
	t.Parallel()

	// Hand-craft a token where mfa_pending is missing — should fall through
	// ParseClaims' "token is not a pending-MFA challenge" branch.
	ta, err := newTestTokenAuth(t, "test-secret-must-be-at-least-32-characters-long")
	require.NoError(t, err)

	// Sentinel IDs >9 so the in-memory JWT claim values don't trip the
	// hermetic-test regex `int64([1-9])` — this token never touches a DB
	// row, the verify path rejects it before any lookup.
	_, tokenString, err := ta.JwtAuth.Encode(map[string]any{
		"account_id": int64(4242),
		"scope":      MFAChallengeScopeTenant,
		"tenant_id":  int64(70010001),
		// mfa_pending intentionally omitted
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Minute).Unix(),
	})
	require.NoError(t, err)

	claims, err := ta.ParseMFAChallengeJWT(tokenString)
	require.Error(t, err)
	assert.Nil(t, claims)
}
