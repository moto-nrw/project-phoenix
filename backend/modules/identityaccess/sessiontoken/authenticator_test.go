package sessiontoken_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken/tokentest"
)

// =============================================================================
// Context Helpers Tests
// =============================================================================

func TestClaimsFromCtx_ValidClaims(t *testing.T) {
	t.Parallel()

	claims := sessiontoken.AppClaims{
		ID:          42,
		Sub:         "user@example.com",
		Username:    "johndoe",
		Roles:       []string{"admin"},
		Permissions: []string{"read", "write"},
		IsAdmin:     true,
	}

	ctx := context.WithValue(context.Background(), sessiontoken.CtxClaims, claims)

	result := sessiontoken.ClaimsFromCtx(ctx)
	assert.Equal(t, 42, result.ID)
	assert.Equal(t, "user@example.com", result.Sub)
	assert.Equal(t, "johndoe", result.Username)
	assert.Equal(t, []string{"admin"}, result.Roles)
	assert.Equal(t, []string{"read", "write"}, result.Permissions)
	assert.True(t, result.IsAdmin)
}

func TestClaimsFromCtx_NoClaims(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	result := sessiontoken.ClaimsFromCtx(ctx)
	assert.Equal(t, sessiontoken.AppClaims{}, result)
	assert.Equal(t, 0, result.ID)
	assert.Empty(t, result.Sub)
}

func TestClaimsFromCtx_WrongType(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), sessiontoken.CtxClaims, "not a claims struct")

	result := sessiontoken.ClaimsFromCtx(ctx)
	assert.Equal(t, sessiontoken.AppClaims{}, result)
}

func TestActorAccountIDFromCtx(t *testing.T) {
	t.Parallel()

	t.Run("missing claims", func(t *testing.T) {
		assert.Nil(t, sessiontoken.ActorAccountIDFromCtx(context.Background()))
	})

	t.Run("zero account id", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), sessiontoken.CtxClaims, sessiontoken.AppClaims{})
		assert.Nil(t, sessiontoken.ActorAccountIDFromCtx(ctx))
	})

	t.Run("valid account id", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: 42})
		accountID := sessiontoken.ActorAccountIDFromCtx(ctx)
		require.NotNil(t, accountID)
		assert.Equal(t, int64(42), *accountID)
	})
}

func TestPermissionsFromCtx_ValidPermissions(t *testing.T) {
	t.Parallel()

	permissions := []string{"read", "write", "delete"}
	ctx := context.WithValue(context.Background(), sessiontoken.CtxPermissions, permissions)

	result := sessiontoken.PermissionsFromCtx(ctx)
	assert.Equal(t, []string{"read", "write", "delete"}, result)
}

func TestPermissionsFromCtx_NoPermissions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	result := sessiontoken.PermissionsFromCtx(ctx)
	assert.Equal(t, []string{}, result)
}

func TestPermissionsFromCtx_WrongType(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), sessiontoken.CtxPermissions, "not a slice")

	result := sessiontoken.PermissionsFromCtx(ctx)
	assert.Equal(t, []string{}, result)
}

func TestPermissionsFromCtx_EmptySlice(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), sessiontoken.CtxPermissions, []string{})

	result := sessiontoken.PermissionsFromCtx(ctx)
	assert.Equal(t, []string{}, result)
}

func TestRefreshTokenFromCtx_ValidToken(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), sessiontoken.CtxRefreshToken, "refresh-token-value")

	result := sessiontoken.RefreshTokenFromCtx(ctx)
	assert.Equal(t, "refresh-token-value", result)
}

func TestRefreshTokenFromCtx_NoToken(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	result := sessiontoken.RefreshTokenFromCtx(ctx)
	assert.Empty(t, result)
}

func TestRefreshTokenFromCtx_WrongType(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), sessiontoken.CtxRefreshToken, 12345)

	result := sessiontoken.RefreshTokenFromCtx(ctx)
	assert.Empty(t, result)
}

// =============================================================================
// extractBearerToken Tests
// =============================================================================

// =============================================================================
// Authenticator Middleware Tests
// =============================================================================

func TestAuthenticator_ValidToken(t *testing.T) {
	t.Parallel()

	auth, err := newTestTokenAuth(t, testSecret)
	require.NoError(t, err)

	// Create a valid token
	claims := sessiontoken.AppClaims{
		ID:          42,
		Sub:         "user@example.com",
		Roles:       []string{"user"},
		Permissions: []string{"read"},
	}
	token, err := tokentest.CreateJWT(auth, claims)
	require.NoError(t, err)

	// Setup router with middleware
	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(sessiontoken.Authenticator)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		// Verify claims are in context
		ctxClaims := sessiontoken.ClaimsFromCtx(r.Context())
		assert.Equal(t, 42, ctxClaims.ID)
		assert.Equal(t, "user@example.com", ctxClaims.Sub)

		// Verify permissions are in context
		perms := sessiontoken.PermissionsFromCtx(r.Context())
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
	t.Parallel()

	auth, err := newTestTokenAuth(t, testSecret)
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(sessiontoken.Authenticator)
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
	t.Parallel()

	auth, err := newTestTokenAuth(t, testSecret)
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(sessiontoken.Authenticator)
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
	t.Parallel()
	// Set very short expiry

	auth, err := newTestTokenAuth(t, testSecret, time.Millisecond, 24*time.Hour)
	require.NoError(t, err)

	// Create token
	claims := sessiontoken.AppClaims{
		ID:    1,
		Sub:   "test@test.com",
		Roles: []string{"user"},
	}
	token, err := tokentest.CreateJWT(auth, claims)
	require.NoError(t, err)

	// Wait for expiry
	time.Sleep(10 * time.Millisecond)

	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(sessiontoken.Authenticator)
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
	t.Parallel()

	// Create token with different secret
	auth1, err := newTestTokenAuth(t, "different-secret-32-chars!!!!!!!")
	require.NoError(t, err)

	claims := sessiontoken.AppClaims{
		ID:    1,
		Sub:   "test@test.com",
		Roles: []string{"user"},
	}
	token, err := tokentest.CreateJWT(auth1, claims)
	require.NoError(t, err)

	// Verify with different secret
	auth2, err := newTestTokenAuth(t, testSecret)
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Use(auth2.Verifier())
	r.Use(sessiontoken.Authenticator)
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
	t.Parallel()

	auth, err := newTestTokenAuth(t, testSecret)
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
	r.Use(sessiontoken.Authenticator)
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
	t.Parallel()

	auth, err := newTestTokenAuth(t, testSecret)
	require.NoError(t, err)

	// Create a valid refresh token
	claims := sessiontoken.RefreshClaims{
		ID:    42,
		Token: "unique-token-id",
	}
	token, err := tokentest.CreateRefreshJWT(auth, claims)
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(sessiontoken.AuthenticateRefreshJWT)
	r.Post("/refresh", func(w http.ResponseWriter, r *http.Request) {
		// Verify refresh token is in context
		refreshToken := sessiontoken.RefreshTokenFromCtx(r.Context())
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
	t.Parallel()

	auth, err := newTestTokenAuth(t, testSecret)
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(sessiontoken.AuthenticateRefreshJWT)
	r.Post("/refresh", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestAuthenticateRefreshJWT_InvalidToken(t *testing.T) {
	t.Parallel()

	auth, err := newTestTokenAuth(t, testSecret)
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(sessiontoken.AuthenticateRefreshJWT)
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
	t.Parallel()

	auth, err := newTestTokenAuth(t, testSecret)
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
	r.Use(sessiontoken.AuthenticateRefreshJWT)
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
	t.Parallel()

	// Ensure context keys are distinct
	assert.NotEqual(t, sessiontoken.CtxClaims, sessiontoken.CtxRefreshToken)
	assert.NotEqual(t, sessiontoken.CtxClaims, sessiontoken.CtxPermissions)
	assert.NotEqual(t, sessiontoken.CtxRefreshToken, sessiontoken.CtxPermissions)
}

// =============================================================================
// Integration Tests - Full Request Lifecycle
// =============================================================================

func TestFullRequestLifecycle_AccessToken(t *testing.T) {
	t.Parallel()

	auth, err := newTestTokenAuth(t, testSecret)
	require.NoError(t, err)

	// Create claims
	accessClaims := sessiontoken.AppClaims{
		ID:          123,
		Sub:         "user@example.com",
		Username:    "testuser",
		FirstName:   "Test",
		LastName:    "User",
		Roles:       []string{"admin", "user"},
		Permissions: []string{"read", "write", "delete"},
		IsAdmin:     true,
	}

	refreshClaims := sessiontoken.RefreshClaims{
		ID:    123,
		Token: "refresh-token-id",
	}

	// Generate tokens
	accessToken, refreshToken, err := tokentest.GenTokenPair(auth, accessClaims, refreshClaims)
	require.NoError(t, err)
	require.NotEmpty(t, accessToken)
	require.NotEmpty(t, refreshToken)

	// Test access token
	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(sessiontoken.Authenticator)
	r.Get("/protected", func(w http.ResponseWriter, r *http.Request) {
		claims := sessiontoken.ClaimsFromCtx(r.Context())
		perms := sessiontoken.PermissionsFromCtx(r.Context())

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

// =============================================================================
// Error Types Tests
// =============================================================================

func TestErrorTypes(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "token unauthorized", sessiontoken.ErrTokenUnauthorized.Error())
	assert.Equal(t, "token expired", sessiontoken.ErrTokenExpired.Error())
	assert.Equal(t, "invalid access token", sessiontoken.ErrInvalidAccessToken.Error())
}

// =============================================================================
// ErrResponse Tests
// =============================================================================

func TestErrResponse_Render(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	errResp := &sessiontoken.ErrResponse{
		Err:            sessiontoken.ErrTokenExpired,
		HTTPStatusCode: http.StatusUnauthorized,
		Detail:         "token expired",
	}

	err := errResp.Render(rr, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, errResp.Status, "the envelope status is the HTTP status number")
}

func TestErrUnauthorized(t *testing.T) {
	t.Parallel()

	renderer := sessiontoken.ErrUnauthorized(sessiontoken.ErrTokenExpired)
	assert.NotNil(t, renderer)

	errResp, ok := renderer.(*sessiontoken.ErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusUnauthorized, errResp.HTTPStatusCode)
	assert.Equal(t, "general.permission", errResp.Code)
	assert.Equal(t, "token expired", errResp.Detail)
}

// =============================================================================
// Context Value Isolation Tests
// =============================================================================

func TestContextValueIsolation(t *testing.T) {
	t.Parallel()

	// Test that different context keys don't interfere with each other
	claims := sessiontoken.AppClaims{ID: 42}
	permissions := []string{"read"}
	refreshToken := "refresh-token"

	ctx := context.Background()
	ctx = context.WithValue(ctx, sessiontoken.CtxClaims, claims)
	ctx = context.WithValue(ctx, sessiontoken.CtxPermissions, permissions)
	ctx = context.WithValue(ctx, sessiontoken.CtxRefreshToken, refreshToken)

	// All values should be independently retrievable
	assert.Equal(t, 42, sessiontoken.ClaimsFromCtx(ctx).ID)
	assert.Equal(t, []string{"read"}, sessiontoken.PermissionsFromCtx(ctx))
	assert.Equal(t, "refresh-token", sessiontoken.RefreshTokenFromCtx(ctx))
}

// =============================================================================
// Middleware Chain Tests
// =============================================================================

func TestMiddlewareChain_VerifierThenAuthenticator(t *testing.T) {
	t.Parallel()

	auth, err := newTestTokenAuth(t, testSecret)
	require.NoError(t, err)

	claims := sessiontoken.AppClaims{
		ID:          1,
		Sub:         "test@test.com",
		Roles:       []string{"user"},
		Permissions: []string{"read"},
	}
	token, err := tokentest.CreateJWT(auth, claims)
	require.NoError(t, err)

	// Test that verifier and authenticator work together
	handlerCalled := false
	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(sessiontoken.Authenticator)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		// Verify jwtauth context is set by verifier
		token, _, err := jwtauth.FromContext(r.Context())
		assert.NoError(t, err)
		assert.NotNil(t, token)

		// Verify our custom context is set by authenticator
		ctxClaims := sessiontoken.ClaimsFromCtx(r.Context())
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
	t.Parallel()
	// Set very short expiry for refresh token

	auth, err := newTestTokenAuth(t, testSecret, 15*time.Minute, time.Millisecond)
	require.NoError(t, err)

	// Create refresh token
	claims := sessiontoken.RefreshClaims{
		ID:    1,
		Token: "test-token",
	}
	token, err := tokentest.CreateRefreshJWT(auth, claims)
	require.NoError(t, err)

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(sessiontoken.AuthenticateRefreshJWT)
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
	t.Parallel()

	// Create router without Verifier middleware - this should result in nil token
	r := chi.NewRouter()
	// Intentionally not using Verifier() to get nil token
	r.Use(sessiontoken.Authenticator)
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
	t.Parallel()

	// Test refresh JWT middleware without verifier - should result in nil token
	r := chi.NewRouter()
	r.Use(sessiontoken.AuthenticateRefreshJWT)
	r.Post("/refresh", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}
