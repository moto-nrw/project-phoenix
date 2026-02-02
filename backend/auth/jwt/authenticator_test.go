package jwt

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Context Helpers Tests
// =============================================================================

func TestClaimsFromCtx_ValidClaims(t *testing.T) {
	claims := AppClaims{
		ID:          42,
		Sub:         "user@example.com",
		Username:    "johndoe",
		Roles:       []string{"admin"},
		Permissions: []string{"read", "write"},
		IsAdmin:     true,
	}

	ctx := context.WithValue(context.Background(), CtxClaims, claims)

	result := ClaimsFromCtx(ctx)
	assert.Equal(t, 42, result.ID)
	assert.Equal(t, "user@example.com", result.Sub)
	assert.Equal(t, "johndoe", result.Username)
	assert.Equal(t, []string{"admin"}, result.Roles)
	assert.Equal(t, []string{"read", "write"}, result.Permissions)
	assert.True(t, result.IsAdmin)
}

func TestClaimsFromCtx_NoClaims(t *testing.T) {
	ctx := context.Background()

	result := ClaimsFromCtx(ctx)
	assert.Equal(t, AppClaims{}, result)
	assert.Equal(t, 0, result.ID)
	assert.Empty(t, result.Sub)
}

func TestClaimsFromCtx_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), CtxClaims, "not a claims struct")

	result := ClaimsFromCtx(ctx)
	assert.Equal(t, AppClaims{}, result)
}

func TestPermissionsFromCtx_ValidPermissions(t *testing.T) {
	permissions := []string{"read", "write", "delete"}
	ctx := context.WithValue(context.Background(), CtxPermissions, permissions)

	result := PermissionsFromCtx(ctx)
	assert.Equal(t, []string{"read", "write", "delete"}, result)
}

func TestPermissionsFromCtx_NoPermissions(t *testing.T) {
	ctx := context.Background()

	result := PermissionsFromCtx(ctx)
	assert.Equal(t, []string{}, result)
}

func TestPermissionsFromCtx_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), CtxPermissions, "not a slice")

	result := PermissionsFromCtx(ctx)
	assert.Equal(t, []string{}, result)
}

func TestPermissionsFromCtx_EmptySlice(t *testing.T) {
	ctx := context.WithValue(context.Background(), CtxPermissions, []string{})

	result := PermissionsFromCtx(ctx)
	assert.Equal(t, []string{}, result)
}

func TestRefreshTokenFromCtx_ValidToken(t *testing.T) {
	ctx := context.WithValue(context.Background(), CtxRefreshToken, "refresh-token-value")

	result := RefreshTokenFromCtx(ctx)
	assert.Equal(t, "refresh-token-value", result)
}

func TestRefreshTokenFromCtx_NoToken(t *testing.T) {
	ctx := context.Background()

	result := RefreshTokenFromCtx(ctx)
	assert.Empty(t, result)
}

func TestRefreshTokenFromCtx_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), CtxRefreshToken, 12345)

	result := RefreshTokenFromCtx(ctx)
	assert.Empty(t, result)
}

// =============================================================================
// extractBearerToken Tests
// =============================================================================

func TestExtractBearerToken_ValidToken(t *testing.T) {
	// Using a clearly fake test token (not a real JWT)
	result := extractBearerToken("Bearer test-token-value-for-unit-test")
	assert.Equal(t, "test-token-value-for-unit-test", result)
}

func TestExtractBearerToken_EmptyHeader(t *testing.T) {
	result := extractBearerToken("")
	assert.Empty(t, result)
}

func TestExtractBearerToken_NoBearer(t *testing.T) {
	// Token without "Bearer " prefix should return empty
	result := extractBearerToken("some-token-without-bearer-prefix")
	assert.Empty(t, result)
}

func TestExtractBearerToken_LowercaseBearer(t *testing.T) {
	result := extractBearerToken("bearer token")
	assert.Empty(t, result, "Should be case-sensitive")
}

func TestExtractBearerToken_BearerNoSpace(t *testing.T) {
	result := extractBearerToken("Bearertoken")
	assert.Empty(t, result)
}

func TestExtractBearerToken_BearerOnly(t *testing.T) {
	result := extractBearerToken("Bearer ")
	assert.Empty(t, result)
}

func TestExtractBearerToken_TooShort(t *testing.T) {
	result := extractBearerToken("Bearer")
	assert.Empty(t, result)
}

func TestExtractBearerToken_ExtraSpaces(t *testing.T) {
	result := extractBearerToken("Bearer  token")
	assert.Equal(t, " token", result, "Should preserve extra spaces in token")
}

// =============================================================================
// Authenticator Middleware Tests
// =============================================================================

func TestAuthenticator_ValidToken(t *testing.T) {
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	auth, err := NewTokenAuthWithSecret(testSecret)
	require.NoError(t, err)

	// Create a valid token
	claims := AppClaims{
		ID:          42,
		Sub:         "user@example.com",
		Roles:       []string{"user"},
		Permissions: []string{"read"},
	}
	token, err := auth.CreateJWT(claims)
	require.NoError(t, err)

	// Setup router with middleware
	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(Authenticator)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		// Verify claims are in context
		ctxClaims := ClaimsFromCtx(r.Context())
		assert.Equal(t, 42, ctxClaims.ID)
		assert.Equal(t, "user@example.com", ctxClaims.Sub)

		// Verify permissions are in context
		perms := PermissionsFromCtx(r.Context())
		assert.Equal(t, []string{"read"}, perms)

		w.WriteHeader(http.StatusOK)
	})

	// Make request
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestAuthenticator_NoToken(t *testing.T) {
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	auth, err := NewTokenAuthWithSecret(testSecret)
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(Authenticator)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// No Authorization header
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuthenticator_InvalidToken(t *testing.T) {
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	auth, err := NewTokenAuthWithSecret(testSecret)
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(Authenticator)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuthenticator_ExpiredToken(t *testing.T) {
	// Set very short expiry
	viper.Set("auth_jwt_expiry", 1*time.Millisecond)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	auth, err := NewTokenAuthWithSecret(testSecret)
	require.NoError(t, err)

	// Create token
	claims := AppClaims{
		ID:    1,
		Sub:   "test@test.com",
		Roles: []string{"user"},
	}
	token, err := auth.CreateJWT(claims)
	require.NoError(t, err)

	// Wait for expiry
	time.Sleep(10 * time.Millisecond)

	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(Authenticator)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuthenticator_WrongSignature(t *testing.T) {
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	// Create token with different secret
	auth1, err := NewTokenAuthWithSecret("different-secret-32-chars!!!!!!!")
	require.NoError(t, err)

	claims := AppClaims{
		ID:    1,
		Sub:   "test@test.com",
		Roles: []string{"user"},
	}
	token, err := auth1.CreateJWT(claims)
	require.NoError(t, err)

	// Verify with different secret
	auth2, err := NewTokenAuthWithSecret(testSecret)
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Use(auth2.Verifier())
	r.Use(Authenticator)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuthenticator_MalformedClaims(t *testing.T) {
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	auth, err := NewTokenAuthWithSecret(testSecret)
	require.NoError(t, err)

	// Create a token with invalid claims structure (missing required fields)
	// Use the JwtAuth directly to create a malformed token
	_, token, err := auth.JwtAuth.Encode(map[string]any{
		"exp": time.Now().Add(15 * time.Minute).Unix(),
		"iat": time.Now().Unix(),
		// Missing: id, sub, roles
	})
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(Authenticator)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// =============================================================================
// AuthenticateRefreshJWT Middleware Tests
// =============================================================================

func TestAuthenticateRefreshJWT_ValidToken(t *testing.T) {
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	auth, err := NewTokenAuthWithSecret(testSecret)
	require.NoError(t, err)

	// Create a valid refresh token
	claims := RefreshClaims{
		ID:    42,
		Token: "unique-token-id",
	}
	token, err := auth.CreateRefreshJWT(claims)
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(AuthenticateRefreshJWT)
	r.Post("/refresh", func(w http.ResponseWriter, r *http.Request) {
		// Verify refresh token is in context
		refreshToken := RefreshTokenFromCtx(r.Context())
		assert.Equal(t, token, refreshToken)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestAuthenticateRefreshJWT_NoToken(t *testing.T) {
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	auth, err := NewTokenAuthWithSecret(testSecret)
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(AuthenticateRefreshJWT)
	r.Post("/refresh", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuthenticateRefreshJWT_InvalidToken(t *testing.T) {
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	auth, err := NewTokenAuthWithSecret(testSecret)
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(AuthenticateRefreshJWT)
	r.Post("/refresh", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuthenticateRefreshJWT_MalformedClaims(t *testing.T) {
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	auth, err := NewTokenAuthWithSecret(testSecret)
	require.NoError(t, err)

	// Create token with missing claims
	_, token, err := auth.JwtAuth.Encode(map[string]any{
		"exp": time.Now().Add(24 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
		// Missing: id, token
	})
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(AuthenticateRefreshJWT)
	r.Post("/refresh", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// =============================================================================
// CtxKey Tests
// =============================================================================

func TestCtxKey_DistinctValues(t *testing.T) {
	// Ensure context keys are distinct
	assert.NotEqual(t, CtxClaims, CtxRefreshToken)
	assert.NotEqual(t, CtxClaims, CtxPermissions)
	assert.NotEqual(t, CtxRefreshToken, CtxPermissions)
}

// =============================================================================
// Integration Tests - Full Request Lifecycle
// =============================================================================

func TestFullRequestLifecycle_AccessToken(t *testing.T) {
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	auth, err := NewTokenAuthWithSecret(testSecret)
	require.NoError(t, err)

	// Create claims
	accessClaims := AppClaims{
		ID:          123,
		Sub:         "user@example.com",
		Username:    "testuser",
		FirstName:   "Test",
		LastName:    "User",
		Roles:       []string{"admin", "user"},
		Permissions: []string{"read", "write", "delete"},
		IsAdmin:     true,
	}

	refreshClaims := RefreshClaims{
		ID:    123,
		Token: "refresh-token-id",
	}

	// Generate tokens
	accessToken, refreshToken, err := auth.GenTokenPair(accessClaims, refreshClaims)
	require.NoError(t, err)
	require.NotEmpty(t, accessToken)
	require.NotEmpty(t, refreshToken)

	// Test access token
	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(Authenticator)
	r.Get("/protected", func(w http.ResponseWriter, r *http.Request) {
		claims := ClaimsFromCtx(r.Context())
		perms := PermissionsFromCtx(r.Context())

		// Verify all claims are accessible
		assert.Equal(t, 123, claims.ID)
		assert.Equal(t, "user@example.com", claims.Sub)
		assert.Equal(t, "testuser", claims.Username)
		assert.Equal(t, "Test", claims.FirstName)
		assert.Equal(t, "User", claims.LastName)
		assert.Equal(t, []string{"admin", "user"}, claims.Roles)
		assert.Equal(t, []string{"read", "write", "delete"}, perms)
		assert.True(t, claims.IsAdmin)

		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// =============================================================================
// renderUnauthorized Tests
// =============================================================================

func TestRenderUnauthorized_SuccessfulRender(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	renderUnauthorized(rr, req, ErrTokenUnauthorized)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// =============================================================================
// Error Types Tests
// =============================================================================

func TestErrorTypes(t *testing.T) {
	assert.Equal(t, "token unauthorized", ErrTokenUnauthorized.Error())
	assert.Equal(t, "token expired", ErrTokenExpired.Error())
	assert.Equal(t, "invalid access token", ErrInvalidAccessToken.Error())
	assert.Equal(t, "invalid refresh token", ErrInvalidRefreshToken.Error())
}

// =============================================================================
// ErrResponse Tests
// =============================================================================

func TestErrResponse_Render(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	errResp := &ErrResponse{
		Err:            ErrTokenExpired,
		HTTPStatusCode: http.StatusUnauthorized,
		StatusText:     "error",
		ErrorText:      "token expired",
	}

	err := errResp.Render(rr, req)
	assert.NoError(t, err)
}

func TestErrUnauthorized(t *testing.T) {
	renderer := ErrUnauthorized(ErrTokenExpired)
	assert.NotNil(t, renderer)

	errResp, ok := renderer.(*ErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusUnauthorized, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.StatusText)
	assert.Equal(t, "token expired", errResp.ErrorText)
}

// =============================================================================
// Context Value Isolation Tests
// =============================================================================

func TestContextValueIsolation(t *testing.T) {
	// Test that different context keys don't interfere with each other
	claims := AppClaims{ID: 42}
	permissions := []string{"read"}
	refreshToken := "refresh-token"

	ctx := context.Background()
	ctx = context.WithValue(ctx, CtxClaims, claims)
	ctx = context.WithValue(ctx, CtxPermissions, permissions)
	ctx = context.WithValue(ctx, CtxRefreshToken, refreshToken)

	// All values should be independently retrievable
	assert.Equal(t, 42, ClaimsFromCtx(ctx).ID)
	assert.Equal(t, []string{"read"}, PermissionsFromCtx(ctx))
	assert.Equal(t, "refresh-token", RefreshTokenFromCtx(ctx))
}

// =============================================================================
// Middleware Chain Tests
// =============================================================================

func TestMiddlewareChain_VerifierThenAuthenticator(t *testing.T) {
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	auth, err := NewTokenAuthWithSecret(testSecret)
	require.NoError(t, err)

	claims := AppClaims{
		ID:          1,
		Sub:         "test@test.com",
		Roles:       []string{"user"},
		Permissions: []string{"read"},
	}
	token, err := auth.CreateJWT(claims)
	require.NoError(t, err)

	// Test that verifier and authenticator work together
	handlerCalled := false
	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(Authenticator)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		// Verify jwtauth context is set by verifier
		token, _, err := jwtauth.FromContext(r.Context())
		assert.NoError(t, err)
		assert.NotNil(t, token)

		// Verify our custom context is set by authenticator
		ctxClaims := ClaimsFromCtx(r.Context())
		assert.Equal(t, 1, ctxClaims.ID)

		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.True(t, handlerCalled)
	assert.Equal(t, http.StatusOK, rr.Code)
}

// =============================================================================
// Expired Token Tests
// =============================================================================

func TestAuthenticateRefreshJWT_ExpiredToken(t *testing.T) {
	// Set very short expiry for refresh token
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 1*time.Millisecond)

	auth, err := NewTokenAuthWithSecret(testSecret)
	require.NoError(t, err)

	// Create refresh token
	claims := RefreshClaims{
		ID:    1,
		Token: "test-token",
	}
	token, err := auth.CreateRefreshJWT(claims)
	require.NoError(t, err)

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(AuthenticateRefreshJWT)
	r.Post("/refresh", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuthenticator_NilToken(t *testing.T) {
	// Create router without Verifier middleware - this should result in nil token
	r := chi.NewRouter()
	// Intentionally not using Verifier() to get nil token
	r.Use(Authenticator)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	// Should return 401 because token is nil (no verifier)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuthenticateRefreshJWT_NilToken(t *testing.T) {
	// Test refresh JWT middleware without verifier - should result in nil token
	r := chi.NewRouter()
	r.Use(AuthenticateRefreshJWT)
	r.Post("/refresh", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}
