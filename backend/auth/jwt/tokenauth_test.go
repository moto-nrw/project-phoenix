package jwt

import (
	"context"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Setup
// =============================================================================

func setupViperDefaults() {
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)
}

const testSecret = "test-secret-32-chars-minimum!!!"

// =============================================================================
// NewTokenAuthWithSecret Tests
// =============================================================================

func TestNewTokenAuthWithSecret_ValidSecret(t *testing.T) {
	setupViperDefaults()

	auth, err := NewTokenAuthWithSecret(testSecret)
	require.NoError(t, err)
	assert.NotNil(t, auth)
	assert.NotNil(t, auth.JwtAuth)
	assert.Equal(t, 15*time.Minute, auth.JwtExpiry)
	assert.Equal(t, 24*time.Hour, auth.JwtRefreshExpiry)
}

func TestNewTokenAuthWithSecret_EmptySecret(t *testing.T) {
	setupViperDefaults()

	// Empty secret still works (security validation is elsewhere)
	auth, err := NewTokenAuthWithSecret("")
	require.NoError(t, err)
	assert.NotNil(t, auth)
}

func TestNewTokenAuthWithSecret_ShortSecret(t *testing.T) {
	setupViperDefaults()

	// Short secret still works but would log warning in NewTokenAuth
	auth, err := NewTokenAuthWithSecret("short")
	require.NoError(t, err)
	assert.NotNil(t, auth)
}

// =============================================================================
// CreateJWT Tests
// =============================================================================

func TestTokenAuth_CreateJWT_ValidClaims(t *testing.T) {
	setupViperDefaults()
	auth, err := NewTokenAuthWithSecret(testSecret)
	require.NoError(t, err)

	claims := AppClaims{
		ID:          42,
		Sub:         "user@example.com",
		Username:    "johndoe",
		FirstName:   "John",
		LastName:    "Doe",
		Roles:       []string{"admin", "user"},
		Permissions: []string{"read", "write"},
		IsAdmin:     true,
	}

	token, err := auth.CreateJWT(claims)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Token should be a valid JWT format (three base64 parts separated by dots)
	parts := countDots(token)
	assert.Equal(t, 2, parts, "JWT should have 3 parts separated by 2 dots")
}

func TestTokenAuth_CreateJWT_SetsExpiry(t *testing.T) {
	setupViperDefaults()
	auth, err := NewTokenAuthWithSecret(testSecret)
	require.NoError(t, err)

	claims := AppClaims{
		ID:    1,
		Sub:   "test@test.com",
		Roles: []string{"user"},
	}

	beforeCreate := time.Now()
	token, err := auth.CreateJWT(claims)
	require.NoError(t, err)

	// Verify token has correct expiry by decoding and checking claims
	decoded, err := auth.JwtAuth.Decode(token)
	require.NoError(t, err)
	require.NotNil(t, decoded)

	// Check expiration is in the future
	expTime := decoded.Expiration()
	assert.True(t, expTime.After(beforeCreate))
	assert.True(t, expTime.Before(beforeCreate.Add(auth.JwtExpiry+time.Minute)))
}

func TestTokenAuth_CreateJWT_MinimalClaims(t *testing.T) {
	setupViperDefaults()
	auth, err := NewTokenAuthWithSecret(testSecret)
	require.NoError(t, err)

	claims := AppClaims{
		ID:    1,
		Sub:   "test@test.com",
		Roles: []string{},
	}

	token, err := auth.CreateJWT(claims)
	require.NoError(t, err)
	assert.NotEmpty(t, token)
}

func TestTokenAuth_CreateJWT_EmptyPermissions(t *testing.T) {
	setupViperDefaults()
	auth, err := NewTokenAuthWithSecret(testSecret)
	require.NoError(t, err)

	claims := AppClaims{
		ID:          1,
		Sub:         "test@test.com",
		Roles:       []string{"user"},
		Permissions: []string{}, // Empty but present
	}

	token, err := auth.CreateJWT(claims)
	require.NoError(t, err)
	assert.NotEmpty(t, token)
}

func TestTokenAuth_CreateJWT_NilPermissions(t *testing.T) {
	setupViperDefaults()
	auth, err := NewTokenAuthWithSecret(testSecret)
	require.NoError(t, err)

	claims := AppClaims{
		ID:          1,
		Sub:         "test@test.com",
		Roles:       []string{"user"},
		Permissions: nil, // nil
	}

	token, err := auth.CreateJWT(claims)
	require.NoError(t, err)
	assert.NotEmpty(t, token)
}

// =============================================================================
// CreateRefreshJWT Tests
// =============================================================================

func TestTokenAuth_CreateRefreshJWT_ValidClaims(t *testing.T) {
	setupViperDefaults()
	auth, err := NewTokenAuthWithSecret(testSecret)
	require.NoError(t, err)

	claims := RefreshClaims{
		ID:    42,
		Token: "unique-refresh-token-id",
	}

	token, err := auth.CreateRefreshJWT(claims)
	require.NoError(t, err)
	assert.NotEmpty(t, token)
}

func TestTokenAuth_CreateRefreshJWT_SetsExpiry(t *testing.T) {
	setupViperDefaults()
	auth, err := NewTokenAuthWithSecret(testSecret)
	require.NoError(t, err)

	claims := RefreshClaims{
		ID:    1,
		Token: "token",
	}

	beforeCreate := time.Now()
	token, err := auth.CreateRefreshJWT(claims)
	require.NoError(t, err)

	// Verify token has correct expiry by decoding and checking claims
	decoded, err := auth.JwtAuth.Decode(token)
	require.NoError(t, err)
	require.NotNil(t, decoded)

	// Check expiration is in the future
	expTime := decoded.Expiration()
	assert.True(t, expTime.After(beforeCreate))
	assert.True(t, expTime.Before(beforeCreate.Add(auth.JwtRefreshExpiry+time.Minute)))
}

// =============================================================================
// GenTokenPair Tests
// =============================================================================

func TestTokenAuth_GenTokenPair_ValidClaims(t *testing.T) {
	setupViperDefaults()
	auth, err := NewTokenAuthWithSecret(testSecret)
	require.NoError(t, err)

	accessClaims := AppClaims{
		ID:    42,
		Sub:   "user@example.com",
		Roles: []string{"user"},
	}

	refreshClaims := RefreshClaims{
		ID:    42,
		Token: "refresh-token-id",
	}

	accessToken, refreshToken, err := auth.GenTokenPair(accessClaims, refreshClaims)
	require.NoError(t, err)
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, refreshToken)
	assert.NotEqual(t, accessToken, refreshToken, "Access and refresh tokens should be different")
}

// =============================================================================
// GetRefreshExpiry Tests
// =============================================================================

func TestTokenAuth_GetRefreshExpiry(t *testing.T) {
	viper.Set("auth_jwt_refresh_expiry", 48*time.Hour)
	auth, err := NewTokenAuthWithSecret(testSecret)
	require.NoError(t, err)

	expiry := auth.GetRefreshExpiry()
	assert.Equal(t, 48*time.Hour, expiry)
}

// =============================================================================
// Verifier Tests
// =============================================================================

func TestTokenAuth_Verifier_ReturnsMiddleware(t *testing.T) {
	setupViperDefaults()
	auth, err := NewTokenAuthWithSecret(testSecret)
	require.NoError(t, err)

	verifier := auth.Verifier()
	assert.NotNil(t, verifier)
}

// =============================================================================
// ParseStructToMap Tests
// =============================================================================

func TestParseStructToMap_AppClaims(t *testing.T) {
	claims := AppClaims{
		ID:          42,
		Sub:         "test@test.com",
		Username:    "johndoe",
		FirstName:   "John",
		LastName:    "Doe",
		Roles:       []string{"admin", "user"},
		Permissions: []string{"read", "write"},
		IsAdmin:     true,
		CommonClaims: CommonClaims{
			ExpiresAt: 1234567890,
			IssuedAt:  1234567800,
		},
	}

	result, err := ParseStructToMap(claims)
	require.NoError(t, err)

	assert.Equal(t, float64(42), result["id"])
	assert.Equal(t, "test@test.com", result["sub"])
	assert.Equal(t, "johndoe", result["username"])
	assert.Equal(t, "John", result["first_name"])
	assert.Equal(t, "Doe", result["last_name"])
	assert.Equal(t, []string{"admin", "user"}, result["roles"])
	assert.Equal(t, []string{"read", "write"}, result["permissions"])
	assert.Equal(t, true, result["is_admin"])
	assert.Equal(t, int64(1234567890), result["exp"])
	assert.Equal(t, int64(1234567800), result["iat"])
}

func TestParseStructToMap_RefreshClaims(t *testing.T) {
	claims := RefreshClaims{
		ID:    42,
		Token: "refresh-token",
		CommonClaims: CommonClaims{
			ExpiresAt: 1234567890,
			IssuedAt:  1234567800,
		},
	}

	result, err := ParseStructToMap(claims)
	require.NoError(t, err)

	assert.Equal(t, float64(42), result["id"])
	assert.Equal(t, "refresh-token", result["token"])
}

func TestParseStructToMap_EmptySlices(t *testing.T) {
	claims := AppClaims{
		ID:          1,
		Sub:         "test@test.com",
		Roles:       []string{},
		Permissions: []string{},
	}

	result, err := ParseStructToMap(claims)
	require.NoError(t, err)

	// Empty slices should be preserved
	assert.NotNil(t, result["roles"])
	assert.NotNil(t, result["permissions"])
}

func TestParseStructToMap_NilSlices(t *testing.T) {
	claims := AppClaims{
		ID:          1,
		Sub:         "test@test.com",
		Roles:       nil,
		Permissions: nil,
	}

	result, err := ParseStructToMap(claims)
	require.NoError(t, err)

	// Nil slices get explicitly set in ParseStructToMap
	assert.NotNil(t, result)
}

// =============================================================================
// randStringBytes Tests
// =============================================================================

func TestRandStringBytes_GeneratesCorrectLength(t *testing.T) {
	lengths := []int{10, 32, 64, 128}

	for _, length := range lengths {
		result := randStringBytes(length)
		assert.Len(t, result, length)
	}
}

func TestRandStringBytes_GeneratesUniqueValues(t *testing.T) {
	results := make(map[string]bool)

	for i := 0; i < 100; i++ {
		result := randStringBytes(32)
		assert.False(t, results[result], "Generated duplicate random string")
		results[result] = true
	}
}

func TestRandStringBytes_ContainsOnlyValidChars(t *testing.T) {
	validChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	result := randStringBytes(1000)

	for _, char := range result {
		assert.Contains(t, validChars, string(char), "Random string contains invalid character")
	}
}

// =============================================================================
// Token Creation and Verification Round-Trip
// =============================================================================

func TestTokenAuth_CreateAndVerify_AccessToken(t *testing.T) {
	setupViperDefaults()
	auth, err := NewTokenAuthWithSecret(testSecret)
	require.NoError(t, err)

	originalClaims := AppClaims{
		ID:          42,
		Sub:         "user@example.com",
		Username:    "johndoe",
		Roles:       []string{"admin"},
		Permissions: []string{"read", "write"},
		IsAdmin:     true,
	}

	token, err := auth.CreateJWT(originalClaims)
	require.NoError(t, err)

	// Verify the token can be decoded
	// This uses the jwtauth library's internal verification
	verifiedToken, err := auth.JwtAuth.Decode(token)
	require.NoError(t, err)
	assert.NotNil(t, verifiedToken)
}

func TestTokenAuth_CreateAndVerify_RefreshToken(t *testing.T) {
	setupViperDefaults()
	auth, err := NewTokenAuthWithSecret(testSecret)
	require.NoError(t, err)

	originalClaims := RefreshClaims{
		ID:    42,
		Token: "unique-refresh-id",
	}

	token, err := auth.CreateRefreshJWT(originalClaims)
	require.NoError(t, err)

	// Verify the token can be decoded
	verifiedToken, err := auth.JwtAuth.Decode(token)
	require.NoError(t, err)
	assert.NotNil(t, verifiedToken)
}

func TestTokenAuth_VerifyWithWrongSecret(t *testing.T) {
	setupViperDefaults()

	// Create token with one secret
	auth1, err := NewTokenAuthWithSecret("secret-one-32-chars-minimum!!!!")
	require.NoError(t, err)

	claims := AppClaims{
		ID:    1,
		Sub:   "test@test.com",
		Roles: []string{"user"},
	}

	token, err := auth1.CreateJWT(claims)
	require.NoError(t, err)

	// Try to verify with different secret
	auth2, err := NewTokenAuthWithSecret("secret-two-32-chars-minimum!!!!")
	require.NoError(t, err)

	_, err = auth2.JwtAuth.Decode(token)
	assert.Error(t, err, "Token should not verify with different secret")
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestTokenAuth_CreateJWT_SpecialCharactersInClaims(t *testing.T) {
	setupViperDefaults()
	auth, err := NewTokenAuthWithSecret(testSecret)
	require.NoError(t, err)

	claims := AppClaims{
		ID:        1,
		Sub:       "user+special@example.com",
		Username:  "user_with-special.chars",
		FirstName: "José",
		LastName:  "O'Connor-Smith",
		Roles:     []string{"role:admin", "role:user"},
	}

	token, err := auth.CreateJWT(claims)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Verify we can decode and the values are preserved
	decoded, err := auth.JwtAuth.Decode(token)
	require.NoError(t, err)
	require.NotNil(t, decoded)

	claimsMap, err := decoded.AsMap(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "José", claimsMap["first_name"])
	assert.Equal(t, "O'Connor-Smith", claimsMap["last_name"])
}

func TestTokenAuth_CreateJWT_EmptyUsername(t *testing.T) {
	setupViperDefaults()
	auth, err := NewTokenAuthWithSecret(testSecret)
	require.NoError(t, err)

	claims := AppClaims{
		ID:       1,
		Sub:      "test@test.com",
		Username: "", // Empty
		Roles:    []string{"user"},
	}

	token, err := auth.CreateJWT(claims)
	require.NoError(t, err)
	assert.NotEmpty(t, token)
}

func TestTokenAuth_CustomExpiry(t *testing.T) {
	viper.Set("auth_jwt_expiry", 5*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 1*time.Hour)

	auth, err := NewTokenAuthWithSecret(testSecret)
	require.NoError(t, err)

	assert.Equal(t, 5*time.Minute, auth.JwtExpiry)
	assert.Equal(t, 1*time.Hour, auth.JwtRefreshExpiry)
}

// Helper function
func countDots(s string) int {
	count := 0
	for _, c := range s {
		if c == '.' {
			count++
		}
	}
	return count
}
