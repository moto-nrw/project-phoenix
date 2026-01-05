package jwt

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/jwtauth/v5"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClaimsFromCtx tests retrieving AppClaims from context
func TestClaimsFromCtx(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected AppClaims
	}{
		{
			name: "valid claims in context",
			ctx: context.WithValue(context.Background(), CtxClaims, AppClaims{
				ID:          123,
				Sub:         "user@example.com",
				Username:    "testuser",
				Roles:       []string{"admin"},
				Permissions: []string{"users:read"},
				IsAdmin:     true,
			}),
			expected: AppClaims{
				ID:          123,
				Sub:         "user@example.com",
				Username:    "testuser",
				Roles:       []string{"admin"},
				Permissions: []string{"users:read"},
				IsAdmin:     true,
			},
		},
		{
			name:     "no claims in context",
			ctx:      context.Background(),
			expected: AppClaims{}, // Zero value
		},
		{
			name:     "wrong type in context",
			ctx:      context.WithValue(context.Background(), CtxClaims, "not-claims"),
			expected: AppClaims{}, // Zero value
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClaimsFromCtx(tt.ctx)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestPermissionsFromCtx tests retrieving permissions from context
func TestPermissionsFromCtx(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected []string
	}{
		{
			name:     "valid permissions in context",
			ctx:      context.WithValue(context.Background(), CtxPermissions, []string{"users:read", "users:write"}),
			expected: []string{"users:read", "users:write"},
		},
		{
			name:     "empty permissions in context",
			ctx:      context.WithValue(context.Background(), CtxPermissions, []string{}),
			expected: []string{},
		},
		{
			name:     "no permissions in context",
			ctx:      context.Background(),
			expected: []string{}, // Empty slice
		},
		{
			name:     "wrong type in context",
			ctx:      context.WithValue(context.Background(), CtxPermissions, "not-permissions"),
			expected: []string{}, // Empty slice
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PermissionsFromCtx(tt.ctx)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestRefreshTokenFromCtx tests retrieving refresh token from context
func TestRefreshTokenFromCtx(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected string
	}{
		{
			name:     "valid refresh token in context",
			ctx:      context.WithValue(context.Background(), CtxRefreshToken, "refresh-token-abc123"),
			expected: "refresh-token-abc123",
		},
		{
			name:     "no refresh token in context",
			ctx:      context.Background(),
			expected: "", // Empty string
		},
		{
			name:     "wrong type in context",
			ctx:      context.WithValue(context.Background(), CtxRefreshToken, 123),
			expected: "", // Empty string
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RefreshTokenFromCtx(tt.ctx)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestExtractBearerToken tests Bearer token extraction from Authorization header
func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name       string
		authHeader string
		expected   string
	}{
		{
			name:       "valid Bearer token",
			authHeader: "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.token",
			expected:   "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.token",
		},
		{
			name:       "Bearer with spaces",
			authHeader: "Bearer    token-with-spaces",
			expected:   "   token-with-spaces", // Preserves what's after "Bearer "
		},
		{
			name:       "no Bearer prefix",
			authHeader: "token-without-bearer",
			expected:   "",
		},
		{
			name:       "empty header",
			authHeader: "",
			expected:   "",
		},
		{
			name:       "Bearer without space",
			authHeader: "Bearertoken",
			expected:   "",
		},
		{
			name:       "lowercase bearer",
			authHeader: "bearer token",
			expected:   "", // Case-sensitive check
		},
		{
			name:       "Bearer only",
			authHeader: "Bearer ",
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractBearerToken(tt.authHeader)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// setupTestAuthenticator creates a TokenAuth instance for testing
func setupTestAuthenticator(t *testing.T) *TokenAuth {
	t.Helper()

	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	auth, err := NewTokenAuthWithSecret(testJWTSecret)
	require.NoError(t, err)
	require.NotNil(t, auth)

	return auth
}

// createTestRequest creates an HTTP request with JWT token
func createTestRequest(t *testing.T, auth *TokenAuth, claims AppClaims) *http.Request {
	t.Helper()

	tokenString, err := auth.CreateJWT(claims)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)

	return req
}

// createTestRefreshRequest creates an HTTP request with refresh token
func createTestRefreshRequest(t *testing.T, auth *TokenAuth, claims RefreshClaims) *http.Request {
	t.Helper()

	tokenString, err := auth.CreateRefreshJWT(claims)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)

	return req
}

// testHandler is a simple handler that verifies claims are in context
func testHandler(t *testing.T, expectedID int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := ClaimsFromCtx(r.Context())
		assert.Equal(t, expectedID, claims.ID, "Claims ID should match")

		permissions := PermissionsFromCtx(r.Context())
		assert.NotNil(t, permissions, "Permissions should be set in context")

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}
}

// TestAuthenticator tests the main JWT authentication middleware
func TestAuthenticator(t *testing.T) {
	auth := setupTestAuthenticator(t)

	t.Run("valid token passes through", func(t *testing.T) {
		claims := AppClaims{
			ID:          123,
			Sub:         "user@example.com",
			Username:    "testuser",
			Roles:       []string{"admin"},
			Permissions: []string{"users:read", "users:write"},
			IsAdmin:     true,
		}

		req := createTestRequest(t, auth, claims)

		// Apply the Verifier middleware first (required before Authenticator)
		verifierMiddleware := auth.Verifier()
		authenticatorMiddleware := Authenticator

		// Create handler chain: Verifier -> Authenticator -> Test Handler
		handler := verifierMiddleware(authenticatorMiddleware(testHandler(t, 123)))

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "OK", rr.Body.String())
	})

	t.Run("valid token with all fields", func(t *testing.T) {
		claims := AppClaims{
			ID:          456,
			Sub:         "admin@example.com",
			Username:    "admin",
			FirstName:   "Admin",
			LastName:    "User",
			Roles:       []string{"admin", "teacher"},
			Permissions: []string{"*"},
			IsAdmin:     true,
			IsTeacher:   true,
		}

		req := createTestRequest(t, auth, claims)

		handler := auth.Verifier()(Authenticator(testHandler(t, 456)))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("no token returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		// No Authorization header

		handler := auth.Verifier()(Authenticator(testHandler(t, 0)))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		assert.Contains(t, rr.Body.String(), "error")
	})

	t.Run("invalid token format returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer invalid-token-format")

		handler := auth.Verifier()(Authenticator(testHandler(t, 0)))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("expired token returns 401", func(t *testing.T) {
		// Create auth with very short expiry
		viper.Set("auth_jwt_expiry", 100*time.Millisecond)
		viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

		shortAuth, err := NewTokenAuthWithSecret(testJWTSecret)
		require.NoError(t, err)

		claims := AppClaims{
			ID:    789,
			Sub:   "expired@example.com",
			Roles: []string{"user"},
		}

		tokenString, err := shortAuth.CreateJWT(claims)
		require.NoError(t, err)

		// Wait for token to expire
		time.Sleep(150 * time.Millisecond)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer "+tokenString)

		handler := shortAuth.Verifier()(Authenticator(testHandler(t, 0)))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		assert.Contains(t, rr.Body.String(), "error")
	})

	t.Run("token without required claims returns 401", func(t *testing.T) {
		// Create a custom token without required claims
		_, customToken, _ := auth.JwtAuth.Encode(map[string]interface{}{
			// Missing "id", "sub", "roles" - required claims
			"username": "nofields",
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer "+customToken)

		handler := auth.Verifier()(Authenticator(testHandler(t, 0)))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("malformed claims returns 401", func(t *testing.T) {
		// Create token with wrong type for 'id' (string instead of int)
		_, customToken, _ := auth.JwtAuth.Encode(map[string]interface{}{
			"id":    "not-an-int", // Wrong type
			"sub":   "user@example.com",
			"roles": []string{"user"},
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer "+customToken)

		handler := auth.Verifier()(Authenticator(testHandler(t, 0)))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}

// TestAuthenticator_ContextValues verifies that middleware sets correct context values
func TestAuthenticator_ContextValues(t *testing.T) {
	auth := setupTestAuthenticator(t)

	claims := AppClaims{
		ID:          999,
		Sub:         "context@example.com",
		Username:    "contextuser",
		FirstName:   "Context",
		LastName:    "Test",
		Roles:       []string{"admin", "moderator"},
		Permissions: []string{"users:read", "users:write", "groups:read"},
		IsAdmin:     true,
		IsTeacher:   false,
	}

	req := createTestRequest(t, auth, claims)

	// Handler that checks all context values
	testContextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check AppClaims in context
		ctxClaims := ClaimsFromCtx(r.Context())
		assert.Equal(t, 999, ctxClaims.ID)
		assert.Equal(t, "context@example.com", ctxClaims.Sub)
		assert.Equal(t, "contextuser", ctxClaims.Username)
		assert.Equal(t, "Context", ctxClaims.FirstName)
		assert.Equal(t, "Test", ctxClaims.LastName)
		assert.Equal(t, []string{"admin", "moderator"}, ctxClaims.Roles)
		assert.Equal(t, []string{"users:read", "users:write", "groups:read"}, ctxClaims.Permissions)
		assert.True(t, ctxClaims.IsAdmin)
		assert.False(t, ctxClaims.IsTeacher)

		// Check permissions separately in context
		ctxPerms := PermissionsFromCtx(r.Context())
		assert.Equal(t, []string{"users:read", "users:write", "groups:read"}, ctxPerms)

		w.WriteHeader(http.StatusOK)
	})

	handler := auth.Verifier()(Authenticator(testContextHandler))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

// TestAuthenticateRefreshJWT tests the refresh token authentication middleware
func TestAuthenticateRefreshJWT(t *testing.T) {
	auth := setupTestAuthenticator(t)

	t.Run("valid refresh token passes through", func(t *testing.T) {
		claims := RefreshClaims{
			ID:    123,
			Token: "refresh-token-abc123",
		}

		req := createTestRefreshRequest(t, auth, claims)

		// Handler that checks refresh token in context
		testRefreshHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := RefreshTokenFromCtx(r.Context())
			assert.NotEmpty(t, token, "Refresh token should be in context")

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		})

		handler := auth.Verifier()(AuthenticateRefreshJWT(testRefreshHandler))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "OK", rr.Body.String())
	})

	t.Run("valid refresh token sets context correctly", func(t *testing.T) {
		claims := RefreshClaims{
			ID:    456,
			Token: "refresh-xyz789",
		}

		tokenString, err := auth.CreateRefreshJWT(claims)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
		req.Header.Set("Authorization", "Bearer "+tokenString)

		testRefreshHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := RefreshTokenFromCtx(r.Context())
			assert.Equal(t, tokenString, token, "Token string should match")

			w.WriteHeader(http.StatusOK)
		})

		handler := auth.Verifier()(AuthenticateRefreshJWT(testRefreshHandler))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("no refresh token returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
		// No Authorization header

		testRefreshHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("Handler should not be called")
		})

		handler := auth.Verifier()(AuthenticateRefreshJWT(testRefreshHandler))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("invalid refresh token format returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
		req.Header.Set("Authorization", "Bearer invalid-refresh-token")

		testRefreshHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("Handler should not be called")
		})

		handler := auth.Verifier()(AuthenticateRefreshJWT(testRefreshHandler))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("expired refresh token returns 401", func(t *testing.T) {
		// Create auth with very short refresh expiry
		viper.Set("auth_jwt_expiry", 15*time.Minute)
		viper.Set("auth_jwt_refresh_expiry", 100*time.Millisecond)

		shortAuth, err := NewTokenAuthWithSecret(testJWTSecret)
		require.NoError(t, err)

		claims := RefreshClaims{
			ID:    789,
			Token: "expired-refresh",
		}

		tokenString, err := shortAuth.CreateRefreshJWT(claims)
		require.NoError(t, err)

		// Wait for token to expire
		time.Sleep(150 * time.Millisecond)

		req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
		req.Header.Set("Authorization", "Bearer "+tokenString)

		testRefreshHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("Handler should not be called")
		})

		handler := shortAuth.Verifier()(AuthenticateRefreshJWT(testRefreshHandler))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("refresh token without required claims returns 401", func(t *testing.T) {
		// Create custom token without required fields
		_, customToken, _ := auth.JwtAuth.Encode(map[string]interface{}{
			// Missing "id" and "token" - required for refresh token
			"username": "invalid",
		})

		req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
		req.Header.Set("Authorization", "Bearer "+customToken)

		testRefreshHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("Handler should not be called")
		})

		handler := auth.Verifier()(AuthenticateRefreshJWT(testRefreshHandler))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("access token used for refresh endpoint returns 401", func(t *testing.T) {
		// Try to use an access token for refresh (wrong token type)
		claims := AppClaims{
			ID:    123,
			Sub:   "user@example.com",
			Roles: []string{"user"},
		}

		req := createTestRequest(t, auth, claims)

		testRefreshHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("Handler should not be called")
		})

		handler := auth.Verifier()(AuthenticateRefreshJWT(testRefreshHandler))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		// Should fail because access token doesn't have "token" claim
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}

// TestAuthenticator_Integration tests realistic authentication flow
func TestAuthenticator_Integration(t *testing.T) {
	auth := setupTestAuthenticator(t)

	// Simulate login: create token pair
	accessClaims := AppClaims{
		ID:          100,
		Sub:         "integration@example.com",
		Username:    "integrationuser",
		FirstName:   "Integration",
		LastName:    "Test",
		Roles:       []string{"teacher"},
		Permissions: []string{"groups:read", "students:read"},
		IsTeacher:   true,
	}

	refreshClaims := RefreshClaims{
		ID:    100,
		Token: "refresh-integration-abc",
	}

	accessToken, refreshToken, err := auth.GenTokenPair(accessClaims, refreshClaims)
	require.NoError(t, err)

	// Test 1: Use access token for protected endpoint
	t.Run("access protected resource with access token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/groups", nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)

		protectedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := ClaimsFromCtx(r.Context())
			assert.Equal(t, 100, claims.ID)
			assert.Equal(t, "integration@example.com", claims.Sub)
			assert.True(t, claims.IsTeacher)

			perms := PermissionsFromCtx(r.Context())
			assert.Contains(t, perms, "groups:read")

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("Protected resource"))
		})

		handler := auth.Verifier()(Authenticator(protectedHandler))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "Protected resource", rr.Body.String())
	})

	// Test 2: Use refresh token for token refresh endpoint
	t.Run("refresh access token with refresh token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
		req.Header.Set("Authorization", "Bearer "+refreshToken)

		refreshHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := RefreshTokenFromCtx(r.Context())
			assert.Equal(t, refreshToken, token)

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("New tokens issued"))
		})

		handler := auth.Verifier()(AuthenticateRefreshJWT(refreshHandler))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "New tokens issued", rr.Body.String())
	})

	// Test 3: Try to use refresh token for protected resource (should fail)
	t.Run("cannot use refresh token for protected resource", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/groups", nil)
		req.Header.Set("Authorization", "Bearer "+refreshToken)

		protectedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("Handler should not be called with refresh token")
		})

		handler := auth.Verifier()(Authenticator(protectedHandler))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		// Should fail because refresh token doesn't have required access token claims
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}

// TestAuthenticator_ConcurrentRequests tests thread safety
func TestAuthenticator_ConcurrentRequests(t *testing.T) {
	auth := setupTestAuthenticator(t)

	claims := AppClaims{
		ID:          200,
		Sub:         "concurrent@example.com",
		Roles:       []string{"user"},
		Permissions: []string{"read"},
	}

	req := createTestRequest(t, auth, claims)

	handler := auth.Verifier()(Authenticator(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate some processing
		ctxClaims := ClaimsFromCtx(r.Context())
		assert.Equal(t, 200, ctxClaims.ID)
		w.WriteHeader(http.StatusOK)
	})))

	// Run 100 concurrent requests
	concurrency := 100
	done := make(chan bool, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusOK, rr.Code)
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < concurrency; i++ {
		<-done
	}
}

// TestRenderUnauthorized tests the error rendering function
func TestRenderUnauthorized(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		expectedMsg string
	}{
		{
			name:        "token unauthorized",
			err:         ErrTokenUnauthorized,
			expectedMsg: "token unauthorized",
		},
		{
			name:        "token expired",
			err:         ErrTokenExpired,
			expectedMsg: "token expired",
		},
		{
			name:        "invalid access token",
			err:         ErrInvalidAccessToken,
			expectedMsg: "invalid access token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rr := httptest.NewRecorder()

			renderUnauthorized(rr, req, tt.err)

			assert.Equal(t, http.StatusUnauthorized, rr.Code)
			assert.Contains(t, rr.Body.String(), tt.expectedMsg)
			assert.Contains(t, rr.Body.String(), "error") // Status field
		})
	}
}

// TestAuthenticator_WithJWTAuthFromContext tests using jwtauth.FromContext
func TestAuthenticator_WithJWTAuthFromContext(t *testing.T) {
	auth := setupTestAuthenticator(t)

	t.Run("token and claims available in handler via jwtauth.FromContext", func(t *testing.T) {
		claims := AppClaims{
			ID:    300,
			Sub:   "fromctx@example.com",
			Roles: []string{"user"},
		}

		req := createTestRequest(t, auth, claims)

		testJWTAuthHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify we can still access raw JWT token via jwtauth
			token, jwtClaims, err := jwtauth.FromContext(r.Context())
			assert.NoError(t, err)
			assert.NotNil(t, token)
			assert.NotNil(t, jwtClaims)

			// Verify we can access our custom AppClaims
			appClaims := ClaimsFromCtx(r.Context())
			assert.Equal(t, 300, appClaims.ID)

			w.WriteHeader(http.StatusOK)
		})

		handler := auth.Verifier()(Authenticator(testJWTAuthHandler))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}
