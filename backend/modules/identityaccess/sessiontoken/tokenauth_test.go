package sessiontoken_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken/tokentest"
)

// =============================================================================
// Test Setup
// =============================================================================

func newTestTokenAuth(_ *testing.T, secret string, durations ...time.Duration) (*sessiontoken.TokenAuth, error) {
	expiry, refreshExpiry := 15*time.Minute, 24*time.Hour
	if len(durations) > 0 {
		expiry = durations[0]
	}
	if len(durations) > 1 {
		refreshExpiry = durations[1]
	}
	return sessiontoken.NewTokenAuthWithDurations(secret, expiry, refreshExpiry)
}

const testSecret = "test-secret-32-chars-minimum!!!"

// =============================================================================
// NewTokenAuthWithSecret Tests
// =============================================================================

func TestNewTokenAuthWithSecret_ValidSecret(t *testing.T) {
	t.Parallel()

	auth, err := newTestTokenAuth(t, testSecret)
	require.NoError(t, err)
	assert.NotNil(t, auth)
	assert.NotNil(t, auth.JwtAuth)
	assert.Equal(t, 15*time.Minute, auth.JwtExpiry)
	assert.Equal(t, 24*time.Hour, auth.JwtRefreshExpiry)
}

func TestNewTokenAuthWithSecret_EmptySecret(t *testing.T) {
	t.Parallel()

	// Empty secret still works (security validation is elsewhere)
	auth, err := newTestTokenAuth(t, "")
	require.NoError(t, err)
	assert.NotNil(t, auth)
}

func TestNewTokenAuthWithSecret_ShortSecret(t *testing.T) {
	t.Parallel()

	// Short secret still works but would log warning in NewTokenAuth
	auth, err := newTestTokenAuth(t, "short")
	require.NoError(t, err)
	assert.NotNil(t, auth)
}

// =============================================================================
// CreateJWT Tests
// =============================================================================

func TestTokenAuth_CreateJWT_SetsExpiry(t *testing.T) {
	t.Parallel()
	auth, err := newTestTokenAuth(t, testSecret)
	require.NoError(t, err)

	claims := sessiontoken.AppClaims{
		ID:    1,
		Sub:   "test@test.com",
		Roles: []string{"user"},
	}

	beforeCreate := time.Now()
	token, err := tokentest.CreateJWT(auth, claims)
	require.NoError(t, err)

	// Verify token has correct expiry by decoding and checking claims
	decoded, err := auth.JwtAuth.Decode(token)
	require.NoError(t, err)
	require.NotNil(t, decoded)

	// Check expiration is in the future
	expTime, ok := decoded.Expiration()
	require.True(t, ok, "token should have expiration")
	assert.True(t, expTime.After(beforeCreate))
	assert.True(t, expTime.Before(beforeCreate.Add(auth.JwtExpiry+time.Minute)))
}

// =============================================================================
// CreateRefreshJWT Tests
// =============================================================================

func TestTokenAuth_CreateRefreshJWT_SetsExpiry(t *testing.T) {
	t.Parallel()
	auth, err := newTestTokenAuth(t, testSecret)
	require.NoError(t, err)

	claims := tokentest.RefreshClaims{
		ID:    1,
		Token: "token",
	}

	beforeCreate := time.Now()
	token, err := tokentest.CreateRefreshJWT(auth, claims)
	require.NoError(t, err)

	// Verify token has correct expiry by decoding and checking claims
	decoded, err := auth.JwtAuth.Decode(token)
	require.NoError(t, err)
	require.NotNil(t, decoded)

	// Check expiration is in the future
	expTime, ok := decoded.Expiration()
	require.True(t, ok, "token should have expiration")
	assert.True(t, expTime.After(beforeCreate))
	assert.True(t, expTime.Before(beforeCreate.Add(auth.JwtRefreshExpiry+time.Minute)))
}

func TestTokenAuth_CreateRefreshJWT_PreservesPersistedExpiry(t *testing.T) {
	t.Parallel()
	auth, err := newTestTokenAuth(t, testSecret)
	require.NoError(t, err)

	expiresAt := time.Now().Add(10 * time.Minute).Unix()
	token, err := tokentest.CreateRefreshJWT(auth, tokentest.RefreshClaims{
		ID:    1,
		Token: "persisted-token",
		CommonClaims: sessiontoken.CommonClaims{
			ExpiresAt: expiresAt,
		},
	})

	require.NoError(t, err)

	decoded, err := auth.JwtAuth.Decode(token)
	require.NoError(t, err)
	expiry, ok := decoded.Expiration()
	require.True(t, ok)
	assert.Equal(t, expiresAt, expiry.Unix())
}

// =============================================================================
// GenTokenPair Tests
// =============================================================================

func TestTokenAuth_GenTokenPair_ValidClaims(t *testing.T) {
	t.Parallel()
	auth, err := newTestTokenAuth(t, testSecret)
	require.NoError(t, err)

	accessClaims := sessiontoken.AppClaims{
		ID:    42,
		Sub:   "user@example.com",
		Roles: []string{"user"},
	}

	refreshClaims := tokentest.RefreshClaims{
		ID:    42,
		Token: "refresh-token-id",
	}

	accessToken, refreshToken, err := tokentest.GenTokenPair(auth, accessClaims, refreshClaims)
	require.NoError(t, err)
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, refreshToken)
	assert.NotEqual(t, accessToken, refreshToken, "Access and refresh tokens should be different")
}

// =============================================================================
// GetRefreshExpiry Tests
// =============================================================================

func TestTokenAuth_GetRefreshExpiry(t *testing.T) {
	t.Parallel()
	auth, err := newTestTokenAuth(t, testSecret, 15*time.Minute, 48*time.Hour)
	require.NoError(t, err)

	expiry := auth.JwtRefreshExpiry
	assert.Equal(t, 48*time.Hour, expiry)
}

// =============================================================================
// Token Creation and Verification Round-Trip
// =============================================================================

func TestTokenAuth_VerifyWithWrongSecret(t *testing.T) {
	t.Parallel()

	// Create token with one secret
	auth1, err := newTestTokenAuth(t, "secret-one-32-chars-minimum!!!!")
	require.NoError(t, err)

	claims := sessiontoken.AppClaims{
		ID:    1,
		Sub:   "test@test.com",
		Roles: []string{"user"},
	}

	token, err := tokentest.CreateJWT(auth1, claims)
	require.NoError(t, err)

	// Try to verify with different secret
	auth2, err := newTestTokenAuth(t, "secret-two-32-chars-minimum!!!!")
	require.NoError(t, err)

	_, err = auth2.JwtAuth.Decode(token)
	assert.Error(t, err, "Token should not verify with different secret")
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestTokenAuth_CreateJWT_SpecialCharactersInClaims(t *testing.T) {
	t.Parallel()
	auth, err := newTestTokenAuth(t, testSecret)
	require.NoError(t, err)

	claims := sessiontoken.AppClaims{
		ID:        1,
		Sub:       "user+special@example.com",
		Username:  "user_with-special.chars",
		FirstName: "José",
		LastName:  "O'Connor-Smith",
		Roles:     []string{"role:admin", "role:user"},
	}

	token, err := tokentest.CreateJWT(auth, claims)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Verify we can decode and the values are preserved
	decoded, err := auth.JwtAuth.Decode(token)
	require.NoError(t, err)
	require.NotNil(t, decoded)

	var firstName, lastName string
	require.NoError(t, decoded.Get("first_name", &firstName))
	require.NoError(t, decoded.Get("last_name", &lastName))
	assert.Equal(t, "José", firstName)
	assert.Equal(t, "O'Connor-Smith", lastName)
}

func TestTokenAuth_CustomExpiry(t *testing.T) {
	t.Parallel()

	auth, err := newTestTokenAuth(t, testSecret, 5*time.Minute, time.Hour)
	require.NoError(t, err)

	assert.Equal(t, 5*time.Minute, auth.JwtExpiry)
	assert.Equal(t, 1*time.Hour, auth.JwtRefreshExpiry)
}
