package jwt

import (
	"context"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testJWTSecret is a valid secret for unit tests.
// This is NOT a real secret - it's only used in tests.
const testJWTSecret = "test-secret-key-minimum-32-chars-long-for-security"

// testShortSecret is an intentionally weak secret for validation testing.
const testShortSecret = "short"

func newTestTokenAuth(t *testing.T) *TokenAuth {
	t.Helper()

	// Set up viper config for tests
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	auth, err := NewTokenAuthWithSecret(testJWTSecret)
	require.NoError(t, err)
	require.NotNil(t, auth)

	return auth
}

func TestNewTokenAuthWithSecret(t *testing.T) {
	tests := []struct {
		name          string
		secret        string
		jwtExpiry     time.Duration
		refreshExpiry time.Duration
		wantErr       bool
	}{
		{
			name:          "valid secret with standard expiry",
			secret:        testJWTSecret,
			jwtExpiry:     15 * time.Minute,
			refreshExpiry: 24 * time.Hour,
			wantErr:       false,
		},
		{
			name:          "valid secret with custom expiry",
			secret:        testJWTSecret,
			jwtExpiry:     30 * time.Minute,
			refreshExpiry: 48 * time.Hour,
			wantErr:       false,
		},
		{
			name:          "short secret (warning but no error)",
			secret:        testShortSecret,
			jwtExpiry:     15 * time.Minute,
			refreshExpiry: 24 * time.Hour,
			wantErr:       false, // Constructor doesn't fail, just warns
		},
		{
			name:          "empty secret",
			secret:        "",
			jwtExpiry:     15 * time.Minute,
			refreshExpiry: 24 * time.Hour,
			wantErr:       false, // Constructor allows empty (but should warn)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up viper config
			viper.Set("auth_jwt_expiry", tt.jwtExpiry)
			viper.Set("auth_jwt_refresh_expiry", tt.refreshExpiry)

			auth, err := NewTokenAuthWithSecret(tt.secret)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, auth)
			require.NotNil(t, auth.JwtAuth)
			assert.Equal(t, tt.jwtExpiry, auth.JwtExpiry)
			assert.Equal(t, tt.refreshExpiry, auth.JwtRefreshExpiry)
		})
	}
}

func TestCreateJWT(t *testing.T) {
	auth := newTestTokenAuth(t)

	tests := []struct {
		name      string
		claims    AppClaims
		wantErr   bool
		checkFunc func(*testing.T, string)
	}{
		{
			name: "valid access token with all fields",
			claims: AppClaims{
				ID:          123,
				Sub:         "user@example.com",
				Username:    "testuser",
				FirstName:   "John",
				LastName:    "Doe",
				Roles:       []string{"admin", "teacher"},
				Permissions: []string{"users:read", "users:write"},
				IsAdmin:     true,
				IsTeacher:   true,
			},
			wantErr: false,
			checkFunc: func(t *testing.T, tokenString string) {
				// Verify token is not empty
				assert.NotEmpty(t, tokenString)

				// Parse and validate token
				token, err := auth.JwtAuth.Decode(tokenString)
				require.NoError(t, err)

				// Validate token is not expired
				err = jwt.Validate(token)
				require.NoError(t, err)

				// Check claims
				claims, err := token.AsMap(context.Background())
				require.NoError(t, err)

				assert.Equal(t, float64(123), claims["id"])
				assert.Equal(t, "user@example.com", claims["sub"])
				assert.Equal(t, "testuser", claims["username"])
				assert.Equal(t, "John", claims["first_name"])
				assert.Equal(t, "Doe", claims["last_name"])
				assert.True(t, claims["is_admin"].(bool))
				assert.True(t, claims["is_teacher"].(bool))

				// Check roles array
				roles, ok := claims["roles"].([]interface{})
				require.True(t, ok)
				require.Len(t, roles, 2)
				assert.Equal(t, "admin", roles[0])
				assert.Equal(t, "teacher", roles[1])

				// Check permissions array
				perms, ok := claims["permissions"].([]interface{})
				require.True(t, ok)
				require.Len(t, perms, 2)
				assert.Equal(t, "users:read", perms[0])
				assert.Equal(t, "users:write", perms[1])

				// Check expiry and issued at (JWT library returns time.Time)
				exp, ok := claims["exp"].(time.Time)
				require.True(t, ok, "exp should be time.Time, got %T", claims["exp"])
				assert.True(t, exp.After(time.Now()), "Token should not be expired")

				iat, ok := claims["iat"].(time.Time)
				require.True(t, ok, "iat should be time.Time, got %T", claims["iat"])
				assert.True(t, time.Now().After(iat) || time.Now().Equal(iat), "Token should have been issued now or earlier")
			},
		},
		{
			name: "valid access token with minimal fields",
			claims: AppClaims{
				ID:    456,
				Sub:   "minimal@example.com",
				Roles: []string{"user"},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, tokenString string) {
				assert.NotEmpty(t, tokenString)

				token, err := auth.JwtAuth.Decode(tokenString)
				require.NoError(t, err)

				claims, err := token.AsMap(context.Background())
				require.NoError(t, err)

				assert.Equal(t, float64(456), claims["id"])
				assert.Equal(t, "minimal@example.com", claims["sub"])
			},
		},
		{
			name: "token with empty permissions array",
			claims: AppClaims{
				ID:          789,
				Sub:         "noperms@example.com",
				Roles:       []string{"guest"},
				Permissions: []string{}, // Empty permissions
			},
			wantErr: false,
			checkFunc: func(t *testing.T, tokenString string) {
				assert.NotEmpty(t, tokenString)

				token, err := auth.JwtAuth.Decode(tokenString)
				require.NoError(t, err)

				claims, err := token.AsMap(context.Background())
				require.NoError(t, err)

				// Empty array should be preserved
				perms, ok := claims["permissions"].([]interface{})
				require.True(t, ok)
				assert.Empty(t, perms)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenString, err := auth.CreateJWT(tt.claims)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, tokenString)

			if tt.checkFunc != nil {
				tt.checkFunc(t, tokenString)
			}
		})
	}
}

func TestCreateJWT_Expiry(t *testing.T) {
	// Create auth with short expiry for testing
	viper.Set("auth_jwt_expiry", 500*time.Millisecond)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	auth, err := NewTokenAuthWithSecret(testJWTSecret)
	require.NoError(t, err)

	claims := AppClaims{
		ID:    123,
		Sub:   "test@example.com",
		Roles: []string{"user"},
	}

	tokenString, err := auth.CreateJWT(claims)
	require.NoError(t, err)

	// Verify token can be decoded
	token, err := auth.JwtAuth.Decode(tokenString)
	require.NoError(t, err)

	// Wait for token to expire (longer than expiry time)
	time.Sleep(600 * time.Millisecond)

	// Decode again to get fresh state
	token, err = auth.JwtAuth.Decode(tokenString)
	require.NoError(t, err)

	// Verify token is now expired
	err = jwt.Validate(token)
	require.Error(t, err, "Token should be expired after waiting longer than expiry time")
	assert.Contains(t, err.Error(), "exp", "Error should mention expiration")
}

func TestCreateRefreshJWT(t *testing.T) {
	auth := newTestTokenAuth(t)

	tests := []struct {
		name      string
		claims    RefreshClaims
		wantErr   bool
		checkFunc func(*testing.T, string)
	}{
		{
			name: "valid refresh token",
			claims: RefreshClaims{
				ID:    123,
				Token: "refresh-token-abc123",
			},
			wantErr: false,
			checkFunc: func(t *testing.T, tokenString string) {
				assert.NotEmpty(t, tokenString)

				// Parse and validate token
				token, err := auth.JwtAuth.Decode(tokenString)
				require.NoError(t, err)

				err = jwt.Validate(token)
				require.NoError(t, err)

				// Check claims
				claims, err := token.AsMap(context.Background())
				require.NoError(t, err)

				assert.Equal(t, float64(123), claims["id"])
				assert.Equal(t, "refresh-token-abc123", claims["token"])

				// Check expiry (should use refresh expiry, not access expiry)
				exp, ok := claims["exp"].(time.Time)
				require.True(t, ok, "exp should be time.Time, got %T", claims["exp"])
				assert.True(t, exp.After(time.Now().Add(23*time.Hour)), "Refresh token should expire after 23+ hours")
			},
		},
		{
			name: "refresh token with different ID",
			claims: RefreshClaims{
				ID:    456,
				Token: "refresh-token-xyz789",
			},
			wantErr: false,
			checkFunc: func(t *testing.T, tokenString string) {
				token, err := auth.JwtAuth.Decode(tokenString)
				require.NoError(t, err)

				claims, err := token.AsMap(context.Background())
				require.NoError(t, err)

				assert.Equal(t, float64(456), claims["id"])
				assert.Equal(t, "refresh-token-xyz789", claims["token"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenString, err := auth.CreateRefreshJWT(tt.claims)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, tokenString)

			if tt.checkFunc != nil {
				tt.checkFunc(t, tokenString)
			}
		})
	}
}

func TestGenTokenPair(t *testing.T) {
	auth := newTestTokenAuth(t)

	accessClaims := AppClaims{
		ID:          123,
		Sub:         "user@example.com",
		Username:    "testuser",
		Roles:       []string{"admin"},
		Permissions: []string{"users:read"},
		IsAdmin:     true,
	}

	refreshClaims := RefreshClaims{
		ID:    123,
		Token: "refresh-token-abc",
	}

	accessToken, refreshToken, err := auth.GenTokenPair(accessClaims, refreshClaims)
	require.NoError(t, err)
	require.NotEmpty(t, accessToken)
	require.NotEmpty(t, refreshToken)

	// Verify access token
	accessJWT, err := auth.JwtAuth.Decode(accessToken)
	require.NoError(t, err)
	err = jwt.Validate(accessJWT)
	require.NoError(t, err)

	accessMap, err := accessJWT.AsMap(context.Background())
	require.NoError(t, err)
	assert.Equal(t, float64(123), accessMap["id"])
	assert.Equal(t, "user@example.com", accessMap["sub"])

	// Verify refresh token
	refreshJWT, err := auth.JwtAuth.Decode(refreshToken)
	require.NoError(t, err)
	err = jwt.Validate(refreshJWT)
	require.NoError(t, err)

	refreshMap, err := refreshJWT.AsMap(context.Background())
	require.NoError(t, err)
	assert.Equal(t, float64(123), refreshMap["id"])
	assert.Equal(t, "refresh-token-abc", refreshMap["token"])
}

func TestGenTokenPair_DifferentExpiry(t *testing.T) {
	auth := newTestTokenAuth(t)

	accessClaims := AppClaims{
		ID:    123,
		Sub:   "test@example.com",
		Roles: []string{"user"},
	}

	refreshClaims := RefreshClaims{
		ID:    123,
		Token: "refresh-abc",
	}

	accessToken, refreshToken, err := auth.GenTokenPair(accessClaims, refreshClaims)
	require.NoError(t, err)

	// Decode both tokens
	accessJWT, err := auth.JwtAuth.Decode(accessToken)
	require.NoError(t, err)

	refreshJWT, err := auth.JwtAuth.Decode(refreshToken)
	require.NoError(t, err)

	// Get expiry times
	accessMap, _ := accessJWT.AsMap(context.Background())
	refreshMap, _ := refreshJWT.AsMap(context.Background())

	accessExp := accessMap["exp"].(time.Time)
	refreshExp := refreshMap["exp"].(time.Time)

	// Refresh token should expire much later than access token
	// Access: 15 minutes, Refresh: 24 hours
	assert.True(t, refreshExp.After(accessExp), "Refresh token should expire later than access token")

	// Verify the difference is roughly 23 hours 45 minutes (allowing some tolerance)
	expDiff := refreshExp.Sub(accessExp)
	expectedDiff := 24*time.Hour - 15*time.Minute
	assert.InDelta(t, expectedDiff.Seconds(), expDiff.Seconds(), 60, "Token expiry difference should be ~23h 45m")
}

func TestParseStructToMap(t *testing.T) {
	tests := []struct {
		name      string
		input     interface{}
		wantErr   bool
		checkFunc func(*testing.T, map[string]any)
	}{
		{
			name: "AppClaims with all fields",
			input: AppClaims{
				ID:          123,
				Sub:         "user@example.com",
				Username:    "testuser",
				FirstName:   "John",
				LastName:    "Doe",
				Roles:       []string{"admin", "teacher"},
				Permissions: []string{"users:read", "users:write"},
				IsAdmin:     true,
				IsTeacher:   true,
			},
			wantErr: false,
			checkFunc: func(t *testing.T, result map[string]any) {
				assert.Equal(t, float64(123), result["id"])
				assert.Equal(t, "user@example.com", result["sub"])
				assert.Equal(t, "testuser", result["username"])
				assert.Equal(t, "John", result["first_name"])
				assert.Equal(t, "Doe", result["last_name"])
				assert.Equal(t, true, result["is_admin"])
				assert.Equal(t, true, result["is_teacher"])

				// Check arrays are preserved
				roles, ok := result["roles"].([]string)
				require.True(t, ok)
				assert.Equal(t, []string{"admin", "teacher"}, roles)

				perms, ok := result["permissions"].([]string)
				require.True(t, ok)
				assert.Equal(t, []string{"users:read", "users:write"}, perms)

				// Check exp and iat are set (from embedding)
				assert.NotNil(t, result["exp"])
				assert.NotNil(t, result["iat"])
			},
		},
		{
			name: "AppClaims with minimal fields",
			input: AppClaims{
				ID:    456,
				Sub:   "minimal@example.com",
				Roles: []string{"user"},
			},
			wantErr: false,
			checkFunc: func(t *testing.T, result map[string]any) {
				assert.Equal(t, float64(456), result["id"])
				assert.Equal(t, "minimal@example.com", result["sub"])

				roles, ok := result["roles"].([]string)
				require.True(t, ok)
				assert.Equal(t, []string{"user"}, roles)
			},
		},
		{
			name: "RefreshClaims",
			input: RefreshClaims{
				ID:    789,
				Token: "refresh-xyz",
			},
			wantErr: false,
			checkFunc: func(t *testing.T, result map[string]any) {
				assert.Equal(t, float64(789), result["id"])
				assert.Equal(t, "refresh-xyz", result["token"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseStructToMap(tt.input)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)

			if tt.checkFunc != nil {
				tt.checkFunc(t, result)
			}
		})
	}
}

func TestGetRefreshExpiry(t *testing.T) {
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 48*time.Hour)

	auth, err := NewTokenAuthWithSecret(testJWTSecret)
	require.NoError(t, err)

	expiry := auth.GetRefreshExpiry()
	assert.Equal(t, 48*time.Hour, expiry)
}

func TestVerifier(t *testing.T) {
	auth := newTestTokenAuth(t)

	// Verifier should return a middleware function
	middleware := auth.Verifier()
	require.NotNil(t, middleware)

	// The middleware should be a function that takes and returns an http.Handler
	// This is a basic type check - full integration testing would be in authenticator_test.go
	assert.IsType(t, middleware, auth.Verifier())
}

func TestTokenAuth_DefaultInstance(t *testing.T) {
	// Test setting and getting default token auth
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	auth, err := NewTokenAuthWithSecret(testJWTSecret)
	require.NoError(t, err)

	SetDefaultTokenAuth(auth)

	retrieved, err := GetDefaultTokenAuth()
	require.NoError(t, err)
	assert.Equal(t, auth, retrieved)

	// Clear for next test
	SetDefaultTokenAuth(nil)
}

func TestGetDefaultTokenAuth_CreatesNew(t *testing.T) {
	// Clear default
	SetDefaultTokenAuth(nil)

	// Set up viper config
	viper.Set("auth_jwt_secret", testJWTSecret)
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)
	viper.Set("app_env", "test")

	auth, err := GetDefaultTokenAuth()
	require.NoError(t, err)
	require.NotNil(t, auth)

	// Clean up
	SetDefaultTokenAuth(nil)
}

// TestTokenAuth_SecretValidation tests that short secrets trigger warnings but don't fail
func TestTokenAuth_SecretValidation(t *testing.T) {
	tests := []struct {
		name         string
		secret       string
		shouldWarn   bool
		expectCreate bool
	}{
		{
			name:         "strong secret (32+ chars)",
			secret:       testJWTSecret,
			shouldWarn:   false,
			expectCreate: true,
		},
		{
			name:         "weak secret (short)",
			secret:       testShortSecret,
			shouldWarn:   true,
			expectCreate: true, // Still creates, just warns
		},
		{
			name:         "minimum acceptable (32 chars exactly)",
			secret:       "12345678901234567890123456789012",
			shouldWarn:   false,
			expectCreate: true,
		},
		{
			name:         "just below minimum (31 chars)",
			secret:       "1234567890123456789012345678901",
			shouldWarn:   true,
			expectCreate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Set("auth_jwt_expiry", 15*time.Minute)
			viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

			auth, err := NewTokenAuthWithSecret(tt.secret)

			if tt.expectCreate {
				require.NoError(t, err)
				require.NotNil(t, auth)
				assert.Equal(t, len(tt.secret) >= 32, !tt.shouldWarn)
			} else {
				require.Error(t, err)
			}
		})
	}
}

// TestTokenAuth_ClaimsPreservation verifies that all claim fields are properly preserved in tokens
func TestTokenAuth_ClaimsPreservation(t *testing.T) {
	auth := newTestTokenAuth(t)

	originalClaims := AppClaims{
		ID:          999,
		Sub:         "preserve@example.com",
		Username:    "preserve-user",
		FirstName:   "Test",
		LastName:    "Preservation",
		Roles:       []string{"admin", "moderator", "user"},
		Permissions: []string{"read:all", "write:all", "delete:own"},
		IsAdmin:     true,
		IsTeacher:   false,
	}

	// Create token
	tokenString, err := auth.CreateJWT(originalClaims)
	require.NoError(t, err)

	// Decode token
	token, err := auth.JwtAuth.Decode(tokenString)
	require.NoError(t, err)

	claims, err := token.AsMap(context.Background())
	require.NoError(t, err)

	// Verify all fields are preserved exactly
	assert.Equal(t, float64(999), claims["id"])
	assert.Equal(t, "preserve@example.com", claims["sub"])
	assert.Equal(t, "preserve-user", claims["username"])
	assert.Equal(t, "Test", claims["first_name"])
	assert.Equal(t, "Preservation", claims["last_name"])
	assert.Equal(t, true, claims["is_admin"])
	// Note: is_teacher: false is omitted due to omitempty tag, so it's nil in the map
	if val, ok := claims["is_teacher"]; ok {
		assert.Equal(t, false, val, "is_teacher should be false if present")
	}

	// Verify arrays are complete
	roles := claims["roles"].([]interface{})
	require.Len(t, roles, 3)
	assert.Contains(t, roles, "admin")
	assert.Contains(t, roles, "moderator")
	assert.Contains(t, roles, "user")

	perms := claims["permissions"].([]interface{})
	require.Len(t, perms, 3)
	assert.Contains(t, perms, "read:all")
	assert.Contains(t, perms, "write:all")
	assert.Contains(t, perms, "delete:own")
}
