package jwt

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const enrollmentTestSecret = "test-secret-must-be-at-least-32-characters-long"

// --- MFAEnrollmentClaims.ParseClaims happy + rejection paths -----------------

func TestMFAEnrollmentClaims_TenantRoundTrip(t *testing.T) {
	t.Parallel()

	ta, err := NewTokenAuthWithSecret(enrollmentTestSecret)
	require.NoError(t, err)

	in := MFAEnrollmentClaims{
		AccountID: 4242,
		Scope:     MFAEnrollmentScopeTenant,
		TenantID:  70010001,
	}
	tokenString, err := ta.CreateMFAEnrollmentJWT(in, 5*time.Minute)
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

	var out MFAEnrollmentClaims
	require.NoError(t, out.ParseClaims(raw))
	assert.Equal(t, int64(4242), out.AccountID)
	assert.Equal(t, MFAEnrollmentScopeTenant, out.Scope)
	assert.Equal(t, int64(70010001), out.TenantID)
	assert.True(t, out.MFAEnrollmentPending,
		"every enrollment claim must carry mfa_enrollment_pending=true so middleware can discriminate")
	assert.Greater(t, out.ExpiresAt, time.Now().Unix())
}

func TestMFAEnrollmentClaims_PlatformRoundTrip(t *testing.T) {
	t.Parallel()

	ta, err := NewTokenAuthWithSecret(enrollmentTestSecret)
	require.NoError(t, err)

	tokenString, err := ta.CreateMFAEnrollmentJWT(MFAEnrollmentClaims{
		AccountID: 7,
		Scope:     MFAEnrollmentScopePlatform,
	}, 5*time.Minute)
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

	var out MFAEnrollmentClaims
	require.NoError(t, out.ParseClaims(raw))
	assert.Equal(t, MFAEnrollmentScopePlatform, out.Scope)
	assert.Equal(t, int64(0), out.TenantID, "platform enrollment tokens omit tenant_id")
}

func TestMFAEnrollmentClaims_RejectsMissingAccountID(t *testing.T) {
	t.Parallel()

	var c MFAEnrollmentClaims
	err := c.ParseClaims(map[string]any{
		"mfa_enrollment_pending": true,
		"scope":                  MFAEnrollmentScopeTenant,
	})
	assert.Error(t, err, "missing account_id must fail closed")
}

func TestMFAEnrollmentClaims_RejectsBadScope(t *testing.T) {
	t.Parallel()

	var c MFAEnrollmentClaims
	err := c.ParseClaims(map[string]any{
		"account_id":             float64(4242),
		"mfa_enrollment_pending": true,
		"scope":                  "bogus",
	})
	assert.Error(t, err, "unknown scope values must be rejected, not treated as tenant")
}

func TestMFAEnrollmentClaims_RejectsNonPendingToken(t *testing.T) {
	t.Parallel()

	var c MFAEnrollmentClaims
	err := c.ParseClaims(map[string]any{
		"account_id":             float64(4242),
		"scope":                  MFAEnrollmentScopeTenant,
		"mfa_enrollment_pending": false,
	})
	assert.Error(t, err, "tokens missing the pending flag must not parse as enrollment claims")
}

// --- ParseMFAEnrollmentJWT round-trip + edge cases ---------------------------

func TestParseMFAEnrollmentJWT_TenantRoundTrip(t *testing.T) {
	t.Parallel()

	ta, err := NewTokenAuthWithSecret(enrollmentTestSecret)
	require.NoError(t, err)

	tokenString, err := ta.CreateMFAEnrollmentJWT(MFAEnrollmentClaims{
		AccountID: 4242,
		Scope:     MFAEnrollmentScopeTenant,
		TenantID:  70010001,
	}, 5*time.Minute)
	require.NoError(t, err)

	claims, err := ta.ParseMFAEnrollmentJWT(tokenString)
	require.NoError(t, err)
	assert.Equal(t, int64(4242), claims.AccountID)
	assert.Equal(t, int64(70010001), claims.TenantID)
	assert.Equal(t, MFAEnrollmentScopeTenant, claims.Scope)
}

func TestParseMFAEnrollmentJWT_RejectsGarbage(t *testing.T) {
	t.Parallel()

	ta, err := NewTokenAuthWithSecret(enrollmentTestSecret)
	require.NoError(t, err)
	_, err = ta.ParseMFAEnrollmentJWT("not-a-jwt")
	assert.Error(t, err)
}

func TestParseMFAEnrollmentJWT_RejectsExpired(t *testing.T) {
	t.Parallel()

	ta, err := NewTokenAuthWithSecret(enrollmentTestSecret)
	require.NoError(t, err)

	tokenString, err := ta.CreateMFAEnrollmentJWT(MFAEnrollmentClaims{
		AccountID: 4242,
		Scope:     MFAEnrollmentScopeTenant,
	}, -time.Minute) // already expired
	require.NoError(t, err)

	_, err = ta.ParseMFAEnrollmentJWT(tokenString)
	assert.Error(t, err, "expired enrollment tokens must not parse")
}

func TestParseMFAEnrollmentJWT_RejectsRegularAccessToken(t *testing.T) {
	t.Parallel()

	// A regular access JWT happens to be valid signed JSON but lacks
	// mfa_enrollment_pending — ParseMFAEnrollmentJWT must reject it so a
	// regular session token can never authorize /auth/mfa/enroll/*.
	ta, err := NewTokenAuthWithSecret(enrollmentTestSecret)
	require.NoError(t, err)

	_, tokenString, err := ta.JwtAuth.Encode(map[string]any{
		"id":    float64(4242),
		"sub":   "account:4242",
		"roles": []any{"admin"},
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(time.Minute).Unix(),
	})
	require.NoError(t, err)

	_, err = ta.ParseMFAEnrollmentJWT(tokenString)
	assert.Error(t, err,
		"a regular access token (no mfa_enrollment_pending) must be rejected — defense for Item #1 of the #1430 review")
}

// --- AppClaims defense-in-depth: reject mfa_pending and mfa_enrollment_pending ---

func TestAppClaims_RejectsMFAPendingToken(t *testing.T) {
	t.Parallel()

	// A challenge JWT happens to ALSO have id/sub/roles populated. Without
	// the explicit reject, AppClaims.ParseClaims would accept it and the
	// regular Authenticator would treat it as a fully authenticated
	// session. This guards the Sollte/8 item from the #1430 review.
	var c AppClaims
	err := c.ParseClaims(map[string]any{
		"id":          float64(4242),
		"sub":         "account:4242",
		"roles":       []any{"admin"},
		"mfa_pending": true,
		"iat":         float64(time.Now().Unix()),
		"exp":         float64(time.Now().Add(time.Minute).Unix()),
	})
	assert.Error(t, err, "AppClaims must refuse to parse mfa_pending tokens — defense-in-depth")
}

func TestAppClaims_RejectsMFAEnrollmentPendingToken(t *testing.T) {
	t.Parallel()

	// Same shape as TestAppClaims_RejectsMFAPendingToken but for the new
	// enrollment-pending discriminator (Item #1 of the #1430 review).
	var c AppClaims
	err := c.ParseClaims(map[string]any{
		"id":                     float64(4242),
		"sub":                    "account:4242",
		"roles":                  []any{"admin"},
		"mfa_enrollment_pending": true,
		"iat":                    float64(time.Now().Unix()),
		"exp":                    float64(time.Now().Add(time.Minute).Unix()),
	})
	assert.Error(t, err, "AppClaims must refuse to parse enrollment-pending tokens so they cannot authorize a full session")
}
