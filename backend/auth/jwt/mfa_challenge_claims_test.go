package jwt

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMFAChallengeClaims_TenantRoundTrip(t *testing.T) {
	ta, err := NewTokenAuthWithSecret("test-secret-must-be-at-least-32-characters-long")
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
	ta, err := NewTokenAuthWithSecret("test-secret-must-be-at-least-32-characters-long")
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
	var c MFAChallengeClaims
	err := c.ParseClaims(map[string]any{
		"mfa_pending": true,
		"scope":       MFAChallengeScopeTenant,
	})
	assert.Error(t, err)
}

func TestMFAChallengeClaims_RejectsBadScope(t *testing.T) {
	var c MFAChallengeClaims
	err := c.ParseClaims(map[string]any{
		"account_id":  float64(1),
		"mfa_pending": true,
		"scope":       "bogus",
	})
	assert.Error(t, err)
}

func TestMFAChallengeClaims_RejectsNonPendingToken(t *testing.T) {
	var c MFAChallengeClaims
	err := c.ParseClaims(map[string]any{
		"account_id":  float64(1),
		"scope":       MFAChallengeScopeTenant,
		"mfa_pending": false,
	})
	assert.Error(t, err)
}
