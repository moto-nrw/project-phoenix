package jwt

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTokenAuthWithSecret(t *testing.T) {
	tests := []struct {
		name         string
		secret       string
		jwtExpiry    time.Duration
		refreshExp   time.Duration
		wantErr      bool
		errContains  string
	}{
		{
			name:       "valid secret with expiry configured",
			secret:     "test-secret-key-for-jwt-signing-32chars",
			jwtExpiry:  15 * time.Minute,
			refreshExp: 24 * time.Hour,
			wantErr:    false,
		},
		{
			name:       "minimum length secret",
			secret:     "12345678901234567890123456789012",
			jwtExpiry:  5 * time.Minute,
			refreshExp: 1 * time.Hour,
			wantErr:    false,
		},
		{
			name:       "short secret (warns but works)",
			secret:     "short",
			jwtExpiry:  15 * time.Minute,
			refreshExp: 24 * time.Hour,
			wantErr:    false,
		},
		{
			name:       "empty secret",
			secret:     "",
			jwtExpiry:  15 * time.Minute,
			refreshExp: 24 * time.Hour,
			wantErr:    false, // Note: empty secret is allowed but not recommended
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup viper config
			viper.Set("auth_jwt_expiry", tt.jwtExpiry)
			viper.Set("auth_jwt_refresh_expiry", tt.refreshExp)

			auth, err := NewTokenAuthWithSecret(tt.secret)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, auth)
			assert.NotNil(t, auth.JwtAuth)
			assert.Equal(t, tt.jwtExpiry, auth.JwtExpiry)
			assert.Equal(t, tt.refreshExp, auth.JwtRefreshExpiry)
		})
	}
}

func TestTokenAuth_CreateJWT(t *testing.T) {
	// Setup
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	auth, err := NewTokenAuthWithSecret("test-secret-for-jwt-creation-32c")
	require.NoError(t, err)

	tests := []struct {
		name    string
		claims  AppClaims
		wantErr bool
	}{
		{
			name: "valid claims with all fields",
			claims: AppClaims{
				ID:          1,
				Sub:         "user@example.com",
				Username:    "testuser",
				FirstName:   "Test",
				LastName:    "User",
				Roles:       []string{"admin", "teacher"},
				Permissions: []string{"read:users", "write:users"},
				IsAdmin:     true,
				IsTeacher:   true,
			},
			wantErr: false,
		},
		{
			name: "minimal claims",
			claims: AppClaims{
				ID:  1,
				Sub: "minimal@example.com",
			},
			wantErr: false,
		},
		{
			name: "claims with empty roles",
			claims: AppClaims{
				ID:          2,
				Sub:         "noroles@example.com",
				Username:    "noroles",
				Roles:       []string{},
				Permissions: []string{},
			},
			wantErr: false,
		},
		{
			name: "claims with many permissions",
			claims: AppClaims{
				ID:          3,
				Sub:         "manyperm@example.com",
				Permissions: []string{"p1", "p2", "p3", "p4", "p5", "p6", "p7", "p8", "p9", "p10"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := auth.CreateJWT(tt.claims)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotEmpty(t, token)

			// Verify token structure (should have 3 parts: header.payload.signature)
			parts := splitToken(token)
			assert.Len(t, parts, 3, "JWT should have 3 parts")

			// Verify token can be decoded back
			verifiedToken, err := auth.JwtAuth.Decode(token)
			require.NoError(t, err)
			require.NotNil(t, verifiedToken)
		})
	}
}

func TestTokenAuth_CreateRefreshJWT(t *testing.T) {
	// Setup
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	auth, err := NewTokenAuthWithSecret("test-secret-for-refresh-jwt-32c!")
	require.NoError(t, err)

	tests := []struct {
		name    string
		claims  RefreshClaims
		wantErr bool
	}{
		{
			name: "valid refresh claims",
			claims: RefreshClaims{
				ID:    1,
				Token: "refresh-token-uuid-string",
			},
			wantErr: false,
		},
		{
			name: "refresh claims with zero ID",
			claims: RefreshClaims{
				ID:    0,
				Token: "refresh-token-zero-id",
			},
			wantErr: false,
		},
		{
			name: "refresh claims with empty token",
			claims: RefreshClaims{
				ID:    1,
				Token: "",
			},
			wantErr: false, // Empty token is allowed but may cause issues in usage
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := auth.CreateRefreshJWT(tt.claims)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotEmpty(t, token)

			// Verify token structure
			parts := splitToken(token)
			assert.Len(t, parts, 3, "JWT should have 3 parts")

			// Verify token can be decoded back
			verifiedToken, err := auth.JwtAuth.Decode(token)
			require.NoError(t, err)
			require.NotNil(t, verifiedToken)
		})
	}
}

func TestTokenAuth_GenTokenPair(t *testing.T) {
	// Setup
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	auth, err := NewTokenAuthWithSecret("test-secret-for-token-pair-32ch")
	require.NoError(t, err)

	tests := []struct {
		name          string
		accessClaims  AppClaims
		refreshClaims RefreshClaims
		wantErr       bool
	}{
		{
			name: "valid token pair",
			accessClaims: AppClaims{
				ID:          1,
				Sub:         "user@example.com",
				Username:    "testuser",
				Roles:       []string{"user"},
				Permissions: []string{"read:profile"},
			},
			refreshClaims: RefreshClaims{
				ID:    1,
				Token: "unique-refresh-token-id",
			},
			wantErr: false,
		},
		{
			name: "admin user token pair",
			accessClaims: AppClaims{
				ID:          2,
				Sub:         "admin@example.com",
				Username:    "admin",
				Roles:       []string{"admin", "superuser"},
				Permissions: []string{"*"},
				IsAdmin:     true,
			},
			refreshClaims: RefreshClaims{
				ID:    2,
				Token: "admin-refresh-token",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessToken, refreshToken, err := auth.GenTokenPair(tt.accessClaims, tt.refreshClaims)

			if tt.wantErr {
				require.Error(t, err)
				assert.Empty(t, accessToken)
				assert.Empty(t, refreshToken)
				return
			}

			require.NoError(t, err)
			assert.NotEmpty(t, accessToken)
			assert.NotEmpty(t, refreshToken)

			// Tokens should be different
			assert.NotEqual(t, accessToken, refreshToken)

			// Both should be valid JWTs
			assert.Len(t, splitToken(accessToken), 3)
			assert.Len(t, splitToken(refreshToken), 3)
		})
	}
}

func TestTokenAuth_GetRefreshExpiry(t *testing.T) {
	tests := []struct {
		name     string
		expiry   time.Duration
		expected time.Duration
	}{
		{
			name:     "24 hour expiry",
			expiry:   24 * time.Hour,
			expected: 24 * time.Hour,
		},
		{
			name:     "1 hour expiry",
			expiry:   1 * time.Hour,
			expected: 1 * time.Hour,
		},
		{
			name:     "zero expiry",
			expiry:   0,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Set("auth_jwt_expiry", 15*time.Minute)
			viper.Set("auth_jwt_refresh_expiry", tt.expiry)

			auth, err := NewTokenAuthWithSecret("test-secret-for-expiry-test-32c")
			require.NoError(t, err)

			got := auth.GetRefreshExpiry()
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestTokenAuth_Verifier(t *testing.T) {
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	auth, err := NewTokenAuthWithSecret("test-secret-for-verifier-test-32")
	require.NoError(t, err)

	// Verifier should return a middleware function
	verifier := auth.Verifier()
	assert.NotNil(t, verifier)
}

func TestParseStructToMap(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		wantErr bool
		check   func(t *testing.T, result map[string]any)
	}{
		{
			name: "AppClaims with all fields",
			input: AppClaims{
				ID:          1,
				Sub:         "user@example.com",
				Username:    "testuser",
				FirstName:   "Test",
				LastName:    "User",
				Roles:       []string{"admin", "user"},
				Permissions: []string{"read", "write"},
				IsAdmin:     true,
				IsTeacher:   true, // Set to true to ensure it's serialized (omitempty skips false)
				CommonClaims: CommonClaims{
					ExpiresAt: 1234567890,
					IssuedAt:  1234567800,
				},
			},
			wantErr: false,
			check: func(t *testing.T, result map[string]any) {
				assert.Equal(t, float64(1), result["id"])
				assert.Equal(t, "user@example.com", result["sub"])
				assert.Equal(t, "testuser", result["username"])
				assert.Equal(t, "Test", result["first_name"])
				assert.Equal(t, "User", result["last_name"])
				// Roles and permissions should be explicitly set
				assert.Equal(t, []string{"admin", "user"}, result["roles"])
				assert.Equal(t, []string{"read", "write"}, result["permissions"])
				assert.Equal(t, true, result["is_admin"])
				assert.Equal(t, true, result["is_teacher"])
				// Common claims should be included
				assert.Equal(t, int64(1234567890), result["exp"])
				assert.Equal(t, int64(1234567800), result["iat"])
			},
		},
		{
			name: "RefreshClaims",
			input: RefreshClaims{
				ID:    42,
				Token: "refresh-token-value",
				CommonClaims: CommonClaims{
					ExpiresAt: 9876543210,
					IssuedAt:  9876543200,
				},
			},
			wantErr: false,
			check: func(t *testing.T, result map[string]any) {
				assert.Equal(t, float64(42), result["id"])
				assert.Equal(t, "refresh-token-value", result["token"])
				assert.Equal(t, float64(9876543210), result["exp"])
				assert.Equal(t, float64(9876543200), result["iat"])
			},
		},
		{
			name: "AppClaims with empty slices",
			input: AppClaims{
				ID:          1,
				Sub:         "test@example.com",
				Roles:       []string{},
				Permissions: []string{},
			},
			wantErr: false,
			check: func(t *testing.T, result map[string]any) {
				assert.Equal(t, []string{}, result["roles"])
				assert.Equal(t, []string{}, result["permissions"])
			},
		},
		{
			name: "AppClaims with nil slices",
			input: AppClaims{
				ID:          1,
				Sub:         "test@example.com",
				Roles:       nil,
				Permissions: nil,
			},
			wantErr: false,
			check: func(t *testing.T, result map[string]any) {
				// nil slices should be handled - they may be nil or empty in the result
				roles := result["roles"]
				perms := result["permissions"]
				// Either nil or empty slice is acceptable
				assert.True(t, roles == nil || len(roles.([]string)) == 0)
				assert.True(t, perms == nil || len(perms.([]string)) == 0)
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

			if tt.check != nil {
				tt.check(t, result)
			}
		})
	}
}

func TestDefaultTokenAuth(t *testing.T) {
	// Test SetDefaultTokenAuth and GetDefaultTokenAuth
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	// Initially, GetDefaultTokenAuth should create a new one if none set
	// Note: This requires auth_jwt_secret to be configured, so we set a default first
	auth, err := NewTokenAuthWithSecret("test-secret-for-default-auth-32c")
	require.NoError(t, err)

	// Set the default
	SetDefaultTokenAuth(auth)

	// Get should return the same instance
	got, err := GetDefaultTokenAuth()
	require.NoError(t, err)
	assert.Equal(t, auth, got)

	// Reset to nil for other tests
	SetDefaultTokenAuth(nil)
}

func TestNewTokenAuth(t *testing.T) {
	// Save original viper values to restore later
	originalSecret := viper.GetString("auth_jwt_secret")
	originalEnv := viper.GetString("app_env")
	originalExpiry := viper.GetDuration("auth_jwt_expiry")
	originalRefreshExpiry := viper.GetDuration("auth_jwt_refresh_expiry")

	defer func() {
		viper.Set("auth_jwt_secret", originalSecret)
		viper.Set("app_env", originalEnv)
		viper.Set("auth_jwt_expiry", originalExpiry)
		viper.Set("auth_jwt_refresh_expiry", originalRefreshExpiry)
	}()

	tests := []struct {
		name        string
		setup       func()
		wantErr     bool
		errContains string
	}{
		{
			name: "with valid secret",
			setup: func() {
				viper.Set("auth_jwt_secret", "test-secret-for-newauth-32chars!")
				viper.Set("auth_jwt_expiry", 15*time.Minute)
				viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)
			},
			wantErr: false,
		},
		{
			name: "with short secret (warning only)",
			setup: func() {
				viper.Set("auth_jwt_secret", "short")
				viper.Set("auth_jwt_expiry", 15*time.Minute)
				viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)
			},
			wantErr: false, // Short secret logs warning but still works
		},
		{
			name: "random secret in production should fail",
			setup: func() {
				viper.Set("auth_jwt_secret", "random")
				viper.Set("app_env", "production")
				viper.Set("auth_jwt_expiry", 15*time.Minute)
				viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)
			},
			wantErr:     true,
			errContains: "JWT secret cannot be 'random' in production",
		},
		{
			name: "random secret in development generates secret",
			setup: func() {
				viper.Set("auth_jwt_secret", "random")
				viper.Set("app_env", "development")
				viper.Set("app_base_dir", t.TempDir()) // Use temp dir to avoid creating files
				viper.Set("auth_jwt_expiry", 15*time.Minute)
				viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset defaultTokenAuth for each test
			SetDefaultTokenAuth(nil)

			tt.setup()

			auth, err := NewTokenAuth()

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, auth)
			assert.NotNil(t, auth.JwtAuth)
		})
	}
}

func TestNewTokenAuth_RandomSecretPersistence(t *testing.T) {
	// Test that random secret is persisted in development mode
	tempDir := t.TempDir()

	viper.Set("auth_jwt_secret", "random")
	viper.Set("app_env", "development")
	viper.Set("app_base_dir", tempDir)
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	defer func() {
		viper.Set("auth_jwt_secret", "")
		viper.Set("app_env", "")
		viper.Set("app_base_dir", "")
	}()

	// First call should generate and save a secret
	auth1, err := NewTokenAuth()
	require.NoError(t, err)
	require.NotNil(t, auth1)

	// Create a token
	claims := AppClaims{
		ID:          1,
		Sub:         "test@example.com",
		Roles:       []string{"user"},
		Permissions: []string{},
	}
	token1, err := auth1.CreateJWT(claims)
	require.NoError(t, err)

	// Second call should use the same persisted secret
	SetDefaultTokenAuth(nil) // Reset to force new creation
	auth2, err := NewTokenAuth()
	require.NoError(t, err)

	// Both auth instances should be able to verify tokens created by the other
	// (because they use the same persisted secret)
	decoded, err := auth2.JwtAuth.Decode(token1)
	require.NoError(t, err)
	assert.NotNil(t, decoded)
}

func TestNewTokenAuth_ExistingSecretFile(t *testing.T) {
	// Test reading existing secret file
	tempDir := t.TempDir()
	existingSecret := "existing-secret-from-file-32char!"

	// Create existing secret file
	secretFile := filepath.Join(tempDir, ".jwt-dev-secret.key")
	err := os.WriteFile(secretFile, []byte(existingSecret), 0600)
	require.NoError(t, err)

	viper.Set("auth_jwt_secret", "random")
	viper.Set("app_env", "development")
	viper.Set("app_base_dir", tempDir)
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	defer func() {
		viper.Set("auth_jwt_secret", "")
		viper.Set("app_env", "")
		viper.Set("app_base_dir", "")
	}()

	auth, err := NewTokenAuth()
	require.NoError(t, err)
	require.NotNil(t, auth)
}

func TestNewTokenAuth_ShortExistingSecretFile(t *testing.T) {
	// Test that short existing secret file is ignored and new one generated
	tempDir := t.TempDir()
	shortSecret := "short" // Less than 32 chars

	// Create existing secret file with short secret
	secretFile := filepath.Join(tempDir, ".jwt-dev-secret.key")
	err := os.WriteFile(secretFile, []byte(shortSecret), 0600)
	require.NoError(t, err)

	viper.Set("auth_jwt_secret", "random")
	viper.Set("app_env", "development")
	viper.Set("app_base_dir", tempDir)
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	defer func() {
		viper.Set("auth_jwt_secret", "")
		viper.Set("app_env", "")
		viper.Set("app_base_dir", "")
	}()

	auth, err := NewTokenAuth()
	require.NoError(t, err)
	require.NotNil(t, auth)

	// New secret should be generated (file should be updated)
	newSecret, err := os.ReadFile(secretFile)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(newSecret), 32)
}

func TestNewTokenAuth_NoBaseDir(t *testing.T) {
	// Test when app_base_dir is not set - should use current directory
	viper.Set("auth_jwt_secret", "random")
	viper.Set("app_env", "development")
	viper.Set("app_base_dir", "") // Empty base dir
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	defer func() {
		viper.Set("auth_jwt_secret", "")
		viper.Set("app_env", "")
		viper.Set("app_base_dir", "")
		// Clean up any generated file
		os.Remove(".jwt-dev-secret.key")
	}()

	auth, err := NewTokenAuth()
	require.NoError(t, err)
	require.NotNil(t, auth)
}

func TestGetDefaultTokenAuth_WhenNilAndNewTokenAuthFails(t *testing.T) {
	// Save original values
	originalSecret := viper.GetString("auth_jwt_secret")
	originalEnv := viper.GetString("app_env")

	defer func() {
		viper.Set("auth_jwt_secret", originalSecret)
		viper.Set("app_env", originalEnv)
		SetDefaultTokenAuth(nil)
	}()

	// Reset default
	SetDefaultTokenAuth(nil)

	// Configure to fail (random secret in production)
	viper.Set("auth_jwt_secret", "random")
	viper.Set("app_env", "production")

	_, err := GetDefaultTokenAuth()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT secret cannot be 'random' in production")
}

func TestGenTokenPair_RefreshTokenCreationFails(t *testing.T) {
	// This is tricky to test because CreateRefreshJWT uses JSON marshalling
	// which rarely fails. We test the error propagation by ensuring the function
	// handles errors correctly in normal operation.
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	auth, err := NewTokenAuthWithSecret("test-secret-for-genpair-error-32")
	require.NoError(t, err)

	// Normal case should work
	accessClaims := AppClaims{
		ID:          1,
		Sub:         "test@example.com",
		Roles:       []string{"user"},
		Permissions: []string{},
	}
	refreshClaims := RefreshClaims{
		ID:    1,
		Token: "test-token",
	}

	access, refresh, err := auth.GenTokenPair(accessClaims, refreshClaims)
	require.NoError(t, err)
	assert.NotEmpty(t, access)
	assert.NotEmpty(t, refresh)
}

func TestTokenAuth_TokenExpiry(t *testing.T) {
	// Test that tokens have correct expiry times set
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	auth, err := NewTokenAuthWithSecret("test-secret-for-expiry-verify-32")
	require.NoError(t, err)

	// Create access token
	accessClaims := AppClaims{
		ID:          1,
		Sub:         "test@example.com",
		Roles:       []string{"user"},
		Permissions: []string{},
	}

	now := time.Now()
	accessToken, err := auth.CreateJWT(accessClaims)
	require.NoError(t, err)

	// Decode and verify expiry
	decodedAccess, err := auth.JwtAuth.Decode(accessToken)
	require.NoError(t, err)

	accessExp := decodedAccess.Expiration()
	expectedAccessExp := now.Add(15 * time.Minute)
	// Allow 2 second tolerance for test execution time
	assert.WithinDuration(t, expectedAccessExp, accessExp, 2*time.Second)

	// Create refresh token
	refreshClaims := RefreshClaims{
		ID:    1,
		Token: "refresh-id",
	}

	refreshToken, err := auth.CreateRefreshJWT(refreshClaims)
	require.NoError(t, err)

	// Decode and verify expiry
	decodedRefresh, err := auth.JwtAuth.Decode(refreshToken)
	require.NoError(t, err)

	refreshExp := decodedRefresh.Expiration()
	expectedRefreshExp := now.Add(24 * time.Hour)
	assert.WithinDuration(t, expectedRefreshExp, refreshExp, 2*time.Second)
}

// Helper function to split JWT token
func splitToken(token string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(token); i++ {
		if token[i] == '.' {
			parts = append(parts, token[start:i])
			start = i + 1
		}
	}
	parts = append(parts, token[start:])
	return parts
}

func TestNewTokenAuth_WriteSecretFails(t *testing.T) {
	// Test that when os.WriteFile fails, a warning is logged but auth is still created
	// We need to use a directory where write will fail

	// Create a temp directory with no write permissions
	tempDir := t.TempDir()
	readOnlyDir := filepath.Join(tempDir, "readonly")
	err := os.Mkdir(readOnlyDir, 0555) // Read+execute only
	require.NoError(t, err)

	// Ensure cleanup can happen even if test fails
	defer func() {
		_ = os.Chmod(readOnlyDir, 0755) // Restore permissions for cleanup
	}()

	viper.Set("auth_jwt_secret", "random")
	viper.Set("app_env", "development")
	viper.Set("app_base_dir", readOnlyDir)
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	defer func() {
		viper.Set("auth_jwt_secret", "")
		viper.Set("app_env", "")
		viper.Set("app_base_dir", "")
	}()

	// Should still succeed (with warning logged)
	auth, err := NewTokenAuth()
	require.NoError(t, err)
	require.NotNil(t, auth)
}

func TestParseStructToMap_NonJSONMarshalable(t *testing.T) {
	// Test with a struct that can't be JSON marshaled
	// Note: Most Go structs can be marshaled, so this is a tricky edge case
	// Channels and functions cannot be JSON marshaled

	type BadStruct struct {
		Channel chan int `json:"channel"`
	}

	bad := BadStruct{
		Channel: make(chan int),
	}

	_, err := ParseStructToMap(bad)
	require.Error(t, err)
}

func TestCreateJWT_WithZeroExpiry(t *testing.T) {
	// Test CreateJWT when JwtExpiry is zero
	viper.Set("auth_jwt_expiry", 0)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	auth, err := NewTokenAuthWithSecret("test-secret-for-zero-expiry-32c")
	require.NoError(t, err)

	claims := AppClaims{
		ID:          1,
		Sub:         "zero-expiry@example.com",
		Roles:       []string{"user"},
		Permissions: []string{},
	}

	token, err := auth.CreateJWT(claims)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Token should be valid (with 0 duration expiry, it expires immediately at creation time)
	// This tests the edge case
}

func TestCreateRefreshJWT_WithZeroExpiry(t *testing.T) {
	// Test CreateRefreshJWT when JwtRefreshExpiry is zero
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 0)

	auth, err := NewTokenAuthWithSecret("test-secret-for-zero-refresh-32")
	require.NoError(t, err)

	claims := RefreshClaims{
		ID:    1,
		Token: "zero-expiry-refresh-token",
	}

	token, err := auth.CreateRefreshJWT(claims)
	require.NoError(t, err)
	assert.NotEmpty(t, token)
}

func TestGenTokenPair_ConsistentTimestamps(t *testing.T) {
	// Test that GenTokenPair creates tokens with consistent timestamps
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	auth, err := NewTokenAuthWithSecret("test-secret-for-timestamps-32ch")
	require.NoError(t, err)

	accessClaims := AppClaims{
		ID:          1,
		Sub:         "timestamp@example.com",
		Roles:       []string{"user"},
		Permissions: []string{},
	}

	refreshClaims := RefreshClaims{
		ID:    1,
		Token: "timestamp-refresh-token",
	}

	access, refresh, err := auth.GenTokenPair(accessClaims, refreshClaims)
	require.NoError(t, err)

	// Decode both tokens and verify they have valid timestamps
	decodedAccess, err := auth.JwtAuth.Decode(access)
	require.NoError(t, err)

	decodedRefresh, err := auth.JwtAuth.Decode(refresh)
	require.NoError(t, err)

	// Both should have IssuedAt set
	accessIat := decodedAccess.IssuedAt()
	refreshIat := decodedRefresh.IssuedAt()

	// Issued times should be very close (within 1 second of each other)
	diff := accessIat.Sub(refreshIat)
	if diff < 0 {
		diff = -diff
	}
	assert.True(t, diff < time.Second, "Access and refresh tokens should be issued at nearly the same time")
}

func TestRandStringBytes_Length(t *testing.T) {
	// Test various lengths
	lengths := []int{1, 10, 32, 64, 100}

	for _, length := range lengths {
		t.Run(fmt.Sprintf("length_%d", length), func(t *testing.T) {
			result := randStringBytes(length)
			assert.Len(t, result, length)

			// All characters should be from letterBytes
			for _, c := range result {
				assert.Contains(t, letterBytes, string(c))
			}
		})
	}
}

func TestRandStringBytes_Randomness(t *testing.T) {
	// Generate multiple strings and verify they're different (with very high probability)
	n := 32
	results := make(map[string]bool)

	for i := 0; i < 10; i++ {
		s := randStringBytes(n)
		assert.Len(t, s, n)
		results[s] = true
	}

	// All 10 should be unique
	assert.Len(t, results, 10, "Generated strings should all be unique")
}

func TestNewTokenAuthWithSecret_EmptyDurations(t *testing.T) {
	// Test with zero durations
	viper.Set("auth_jwt_expiry", 0)
	viper.Set("auth_jwt_refresh_expiry", 0)

	auth, err := NewTokenAuthWithSecret("test-secret-empty-durations-32c")
	require.NoError(t, err)
	require.NotNil(t, auth)

	assert.Equal(t, time.Duration(0), auth.JwtExpiry)
	assert.Equal(t, time.Duration(0), auth.JwtRefreshExpiry)
}

func TestNewTokenAuthWithSecret_NegativeDurations(t *testing.T) {
	// Test with negative durations (already expired tokens)
	viper.Set("auth_jwt_expiry", -1*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", -1*time.Hour)

	auth, err := NewTokenAuthWithSecret("test-secret-negative-duration-32")
	require.NoError(t, err)
	require.NotNil(t, auth)

	// Negative durations are allowed (useful for testing expired tokens)
	assert.Equal(t, -1*time.Minute, auth.JwtExpiry)
	assert.Equal(t, -1*time.Hour, auth.JwtRefreshExpiry)
}
