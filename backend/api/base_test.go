package api

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	customMiddleware "github.com/moto-nrw/project-phoenix/middleware"
)

// =============================================================================
// parseAllowedOrigins Tests
// =============================================================================

func TestParseAllowedOrigins_Empty(t *testing.T) {
	// t.Setenv automatically restores the original value after the test
	t.Setenv("CORS_ALLOWED_ORIGINS", "")

	origins := parseAllowedOrigins()
	assert.Equal(t, []string{"*"}, origins)
}

func TestParseAllowedOrigins_SingleOrigin(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")

	origins := parseAllowedOrigins()
	assert.Equal(t, []string{"http://localhost:3000"}, origins)
}

func TestParseAllowedOrigins_MultipleOrigins(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000, https://example.com, https://app.example.com")

	origins := parseAllowedOrigins()
	assert.Equal(t, []string{"http://localhost:3000", "https://example.com", "https://app.example.com"}, origins)
}

func TestParseAllowedOrigins_TrimsWhitespace(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "  http://localhost:3000  ,  https://example.com  ")

	origins := parseAllowedOrigins()
	assert.Equal(t, []string{"http://localhost:3000", "https://example.com"}, origins)
}

func TestParseAllowedOrigins_WildcardOrigin(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "*")

	origins := parseAllowedOrigins()
	assert.Equal(t, []string{"*"}, origins)
}

func TestParseAllowedOrigins_MixedOriginsWithWildcard(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000, *")

	origins := parseAllowedOrigins()
	assert.Equal(t, []string{"http://localhost:3000", "*"}, origins)
}

func TestParseAllowedOrigins_SingleOriginNoComma(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://production.example.com")

	origins := parseAllowedOrigins()
	assert.Equal(t, []string{"https://production.example.com"}, origins)
}

func TestParseAllowedOrigins_EmptyAfterTrim(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "   ")

	origins := parseAllowedOrigins()
	// After trimming, the string becomes empty elements, but split still returns one element
	assert.Len(t, origins, 1)
}

// =============================================================================
// parsePositiveInt Tests
// =============================================================================

func TestParsePositiveInt_Empty(t *testing.T) {
	t.Setenv("TEST_POSITIVE_INT", "")

	result := parsePositiveInt("TEST_POSITIVE_INT", 42)
	assert.Equal(t, 42, result)
}

func TestParsePositiveInt_ValidValue(t *testing.T) {
	t.Setenv("TEST_POSITIVE_INT", "100")

	result := parsePositiveInt("TEST_POSITIVE_INT", 42)
	assert.Equal(t, 100, result)
}

func TestParsePositiveInt_ZeroReturnsDefault(t *testing.T) {
	t.Setenv("TEST_POSITIVE_INT", "0")

	result := parsePositiveInt("TEST_POSITIVE_INT", 42)
	assert.Equal(t, 42, result)
}

func TestParsePositiveInt_NegativeReturnsDefault(t *testing.T) {
	t.Setenv("TEST_POSITIVE_INT", "-5")

	result := parsePositiveInt("TEST_POSITIVE_INT", 42)
	assert.Equal(t, 42, result)
}

func TestParsePositiveInt_InvalidStringReturnsDefault(t *testing.T) {
	t.Setenv("TEST_POSITIVE_INT", "not_a_number")

	result := parsePositiveInt("TEST_POSITIVE_INT", 42)
	assert.Equal(t, 42, result)
}

func TestParsePositiveInt_FloatReturnsDefault(t *testing.T) {
	t.Setenv("TEST_POSITIVE_INT", "10.5")

	result := parsePositiveInt("TEST_POSITIVE_INT", 42)
	assert.Equal(t, 42, result)
}

func TestParsePositiveInt_LargeValue(t *testing.T) {
	t.Setenv("TEST_POSITIVE_INT", "1000000")

	result := parsePositiveInt("TEST_POSITIVE_INT", 42)
	assert.Equal(t, 1000000, result)
}

func TestParsePositiveInt_OneIsValid(t *testing.T) {
	t.Setenv("TEST_POSITIVE_INT", "1")

	result := parsePositiveInt("TEST_POSITIVE_INT", 42)
	assert.Equal(t, 1, result)
}

func TestParsePositiveInt_WhitespaceInValue(t *testing.T) {
	t.Setenv("TEST_POSITIVE_INT", " 50 ")

	// strconv.Atoi does not trim whitespace
	result := parsePositiveInt("TEST_POSITIVE_INT", 42)
	assert.Equal(t, 42, result)
}

func TestParsePositiveInt_EmptyEnvVar(t *testing.T) {
	// Don't set the env var at all - test with non-existent var
	result := parsePositiveInt("NON_EXISTENT_ENV_VAR_12345", 99)
	assert.Equal(t, 99, result)
}

func TestParsePositiveInt_HexValue(t *testing.T) {
	t.Setenv("TEST_POSITIVE_INT", "0xFF")

	// strconv.Atoi doesn't parse hex
	result := parsePositiveInt("TEST_POSITIVE_INT", 42)
	assert.Equal(t, 42, result)
}

func TestParsePositiveInt_ScientificNotation(t *testing.T) {
	t.Setenv("TEST_POSITIVE_INT", "1e5")

	// strconv.Atoi doesn't parse scientific notation
	result := parsePositiveInt("TEST_POSITIVE_INT", 42)
	assert.Equal(t, 42, result)
}

// =============================================================================
// setupSecurityLogging Tests
// =============================================================================

func TestSetupSecurityLogging_Disabled(t *testing.T) {
	t.Setenv("SECURITY_LOGGING_ENABLED", "")
	router := chi.NewRouter()

	logger := setupSecurityLogging(router)
	assert.Nil(t, logger)
}

func TestSetupSecurityLogging_Enabled(t *testing.T) {
	t.Setenv("SECURITY_LOGGING_ENABLED", "true")

	router := chi.NewRouter()
	logger := setupSecurityLogging(router)
	assert.NotNil(t, logger)
}

func TestSetupSecurityLogging_NotTrueValue(t *testing.T) {
	t.Setenv("SECURITY_LOGGING_ENABLED", "yes")

	router := chi.NewRouter()
	logger := setupSecurityLogging(router)
	assert.Nil(t, logger)
}

func TestSetupSecurityLogging_FalseValue(t *testing.T) {
	t.Setenv("SECURITY_LOGGING_ENABLED", "false")

	router := chi.NewRouter()
	logger := setupSecurityLogging(router)
	assert.Nil(t, logger)
}

func TestSetupSecurityLogging_OneValue(t *testing.T) {
	t.Setenv("SECURITY_LOGGING_ENABLED", "1")

	router := chi.NewRouter()
	logger := setupSecurityLogging(router)
	assert.Nil(t, logger)
}

func TestSetupSecurityLogging_TrueUppercase(t *testing.T) {
	t.Setenv("SECURITY_LOGGING_ENABLED", "TRUE")

	router := chi.NewRouter()
	logger := setupSecurityLogging(router)
	// Only "true" (lowercase) is accepted
	assert.Nil(t, logger)
}

func TestSetupSecurityLogging_MiddlewareApplied(t *testing.T) {
	t.Setenv("SECURITY_LOGGING_ENABLED", "true")

	router := chi.NewRouter()
	logger := setupSecurityLogging(router)
	require.NotNil(t, logger)

	// Add a test route and verify middleware doesn't break the request
	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// =============================================================================
// setupRateLimiting Tests
// =============================================================================

func TestSetupRateLimiting_Disabled(t *testing.T) {
	t.Setenv("RATE_LIMIT_ENABLED", "")
	router := chi.NewRouter()

	// Should not panic or add middleware when disabled
	setupRateLimiting(router, nil)
}

func TestSetupRateLimiting_Enabled(t *testing.T) {
	t.Setenv("RATE_LIMIT_ENABLED", "true")

	router := chi.NewRouter()
	setupRateLimiting(router, nil)

	// Router should have middleware added (verifying no panic)
	require.NotNil(t, router)
}

func TestSetupRateLimiting_CustomValues(t *testing.T) {
	t.Setenv("RATE_LIMIT_ENABLED", "true")
	t.Setenv("RATE_LIMIT_REQUESTS_PER_MINUTE", "120")
	t.Setenv("RATE_LIMIT_BURST", "20")

	router := chi.NewRouter()
	setupRateLimiting(router, nil)

	// Should complete without error
	require.NotNil(t, router)
}

func TestSetupRateLimiting_FalseValue(t *testing.T) {
	t.Setenv("RATE_LIMIT_ENABLED", "false")

	router := chi.NewRouter()
	setupRateLimiting(router, nil)
	// Should not panic
	require.NotNil(t, router)
}

func TestSetupRateLimiting_WithSecurityLogger(t *testing.T) {
	t.Setenv("RATE_LIMIT_ENABLED", "true")

	router := chi.NewRouter()
	securityLogger := customMiddleware.NewSecurityLogger()
	setupRateLimiting(router, securityLogger)

	// Should complete without error and logger should be set
	require.NotNil(t, router)
}

func TestSetupRateLimiting_InvalidCustomValues(t *testing.T) {
	t.Setenv("RATE_LIMIT_ENABLED", "true")
	t.Setenv("RATE_LIMIT_REQUESTS_PER_MINUTE", "invalid")
	t.Setenv("RATE_LIMIT_BURST", "also_invalid")

	router := chi.NewRouter()
	// Should use default values when parsing fails
	setupRateLimiting(router, nil)

	require.NotNil(t, router)
}

func TestSetupRateLimiting_ZeroValues(t *testing.T) {
	t.Setenv("RATE_LIMIT_ENABLED", "true")
	t.Setenv("RATE_LIMIT_REQUESTS_PER_MINUTE", "0")
	t.Setenv("RATE_LIMIT_BURST", "0")

	router := chi.NewRouter()
	// Should use default values for zero
	setupRateLimiting(router, nil)

	require.NotNil(t, router)
}

func TestSetupRateLimiting_NegativeValues(t *testing.T) {
	t.Setenv("RATE_LIMIT_ENABLED", "true")
	t.Setenv("RATE_LIMIT_REQUESTS_PER_MINUTE", "-10")
	t.Setenv("RATE_LIMIT_BURST", "-5")

	router := chi.NewRouter()
	// Should use default values for negative
	setupRateLimiting(router, nil)

	require.NotNil(t, router)
}

func TestSetupRateLimiting_MiddlewareAllowsRequests(t *testing.T) {
	t.Setenv("RATE_LIMIT_ENABLED", "true")
	t.Setenv("RATE_LIMIT_REQUESTS_PER_MINUTE", "60")
	t.Setenv("RATE_LIMIT_BURST", "10")

	router := chi.NewRouter()
	setupRateLimiting(router, nil)

	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// =============================================================================
// setupBasicMiddleware Tests
// =============================================================================

func TestSetupBasicMiddleware(t *testing.T) {
	router := chi.NewRouter()
	setupBasicMiddleware(router)

	// Add a test route
	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Make a request to verify middleware chain works
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSetupBasicMiddleware_SecurityHeadersPresent(t *testing.T) {
	router := chi.NewRouter()
	setupBasicMiddleware(router)

	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Check security headers are set by SecurityHeaders middleware
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
}

func TestSetupBasicMiddleware_RecovererMiddleware(t *testing.T) {
	router := chi.NewRouter()
	setupBasicMiddleware(router)

	// Add a route that panics
	router.Get("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	req := httptest.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()

	// Should not panic due to Recoverer middleware
	assert.NotPanics(t, func() {
		router.ServeHTTP(w, req)
	})

	// Recoverer returns 500 on panic
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSetupBasicMiddleware_AllHTTPMethods(t *testing.T) {
	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			router := chi.NewRouter()
			setupBasicMiddleware(router)

			router.Method(method, "/test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(method, "/test", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

// =============================================================================
// setupCORS Tests
// =============================================================================

func TestSetupCORS(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")

	router := chi.NewRouter()
	setupCORS(router)

	// Add a test route
	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Make an OPTIONS request
	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// CORS middleware should add headers
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Origin"), "http://localhost:3000")
}

func TestSetupCORS_WildcardOrigin(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "*")

	router := chi.NewRouter()
	setupCORS(router)

	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "http://any-origin.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Wildcard should allow any origin
	assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestSetupCORS_DisallowedOrigin(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")

	router := chi.NewRouter()
	setupCORS(router)

	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "http://malicious-site.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Disallowed origin should not be in the response
	allowedOrigin := w.Header().Get("Access-Control-Allow-Origin")
	assert.NotEqual(t, "http://malicious-site.com", allowedOrigin)
}

func TestSetupCORS_AllowedMethods(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")

	router := chi.NewRouter()
	setupCORS(router)

	router.Post("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	allowedMethods := w.Header().Get("Access-Control-Allow-Methods")
	assert.Contains(t, allowedMethods, "POST")
}

func TestSetupCORS_AllowedHeaders(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")

	router := chi.NewRouter()
	setupCORS(router)

	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "Authorization, Content-Type")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	allowedHeaders := w.Header().Get("Access-Control-Allow-Headers")
	assert.Contains(t, allowedHeaders, "Authorization")
	assert.Contains(t, allowedHeaders, "Content-Type")
}

func TestSetupCORS_CustomHeaders(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")

	router := chi.NewRouter()
	setupCORS(router)

	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "X-Staff-PIN, X-Device-Key")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	allowedHeaders := w.Header().Get("Access-Control-Allow-Headers")
	// Note: CORS library normalizes headers to Title-Case (X-Staff-Pin)
	assert.Contains(t, allowedHeaders, "X-Staff-Pin")
	assert.Contains(t, allowedHeaders, "X-Device-Key")
}

func TestSetupCORS_CredentialsAllowed(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")

	router := chi.NewRouter()
	setupCORS(router)

	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// AllowCredentials is true in setupCORS
	assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
}

func TestSetupCORS_ActualGETRequest(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")

	router := chi.NewRouter()
	setupCORS(router)

	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data"))
	})

	// Actual GET request with Origin header
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "data", w.Body.String())
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Origin"), "http://localhost:3000")
}

func TestSetupCORS_MultipleOrigins(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000, https://example.com")

	router := chi.NewRouter()
	setupCORS(router)

	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Request from second origin
	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Contains(t, w.Header().Get("Access-Control-Allow-Origin"), "https://example.com")
}

// =============================================================================
// API ServeHTTP Tests
// =============================================================================

func TestAPI_ServeHTTP(t *testing.T) {
	router := chi.NewRouter()
	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test response"))
	})

	api := &API{
		Router: router,
	}

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	api.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "test response", w.Body.String())
}

func TestAPI_ServeHTTP_NotFound(t *testing.T) {
	router := chi.NewRouter()
	router.Get("/existing", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	api := &API{
		Router: router,
	}

	req := httptest.NewRequest("GET", "/non-existent", nil)
	w := httptest.NewRecorder()
	api.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAPI_ServeHTTP_POST(t *testing.T) {
	router := chi.NewRouter()
	router.Post("/data", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	})

	api := &API{
		Router: router,
	}

	req := httptest.NewRequest("POST", "/data", nil)
	w := httptest.NewRecorder()
	api.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "created", w.Body.String())
}

func TestAPI_ServeHTTP_MethodNotAllowed(t *testing.T) {
	router := chi.NewRouter()
	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	api := &API{
		Router: router,
	}

	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()
	api.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestAPI_ServeHTTP_WithHeaders(t *testing.T) {
	router := chi.NewRouter()
	router.Get("/headers", func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(authHeader))
	})

	api := &API{
		Router: router,
	}

	req := httptest.NewRequest("GET", "/headers", nil)
	req.Header.Set("Authorization", "Bearer token123")
	w := httptest.NewRecorder()
	api.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Bearer token123", w.Body.String())
}

func TestAPI_ServeHTTP_MultipleRoutes(t *testing.T) {
	router := chi.NewRouter()
	router.Get("/one", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("route one"))
	})
	router.Get("/two", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("route two"))
	})

	api := &API{
		Router: router,
	}

	// Test first route
	req1 := httptest.NewRequest("GET", "/one", nil)
	w1 := httptest.NewRecorder()
	api.ServeHTTP(w1, req1)
	assert.Equal(t, "route one", w1.Body.String())

	// Test second route
	req2 := httptest.NewRequest("GET", "/two", nil)
	w2 := httptest.NewRecorder()
	api.ServeHTTP(w2, req2)
	assert.Equal(t, "route two", w2.Body.String())
}

// =============================================================================
// Combined Middleware Tests
// =============================================================================

func TestSetupBasicMiddleware_WithCORS(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")

	router := chi.NewRouter()
	setupBasicMiddleware(router)
	setupCORS(router)

	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Both security headers and CORS should be present
	assert.NotEmpty(t, w.Header().Get("X-Frame-Options"))
	assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestSetupBasicMiddleware_WithRateLimiting(t *testing.T) {
	t.Setenv("RATE_LIMIT_ENABLED", "true")
	t.Setenv("RATE_LIMIT_REQUESTS_PER_MINUTE", "100")
	t.Setenv("RATE_LIMIT_BURST", "10")

	router := chi.NewRouter()
	setupBasicMiddleware(router)
	setupRateLimiting(router, nil)

	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Request should succeed and have security headers
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
}

func TestSetupBasicMiddleware_FullStack(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")
	t.Setenv("RATE_LIMIT_ENABLED", "true")
	t.Setenv("SECURITY_LOGGING_ENABLED", "true")

	router := chi.NewRouter()
	setupBasicMiddleware(router)
	setupCORS(router)
	securityLogger := setupSecurityLogging(router)
	setupRateLimiting(router, securityLogger)

	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("full stack"))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.RemoteAddr = "192.168.1.200:12345"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "full stack", w.Body.String())
}

// =============================================================================
// HTTPS/TLS Tests
// =============================================================================

func TestSetupBasicMiddleware_HSTS_WithTLS(t *testing.T) {
	router := chi.NewRouter()
	setupBasicMiddleware(router)

	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.TLS = &tls.ConnectionState{} // Simulate TLS connection
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// HSTS should be set for TLS connections
	hsts := w.Header().Get("Strict-Transport-Security")
	assert.NotEmpty(t, hsts)
	assert.Contains(t, hsts, "max-age")
}

func TestSetupBasicMiddleware_HSTS_WithXForwardedProto(t *testing.T) {
	router := chi.NewRouter()
	setupBasicMiddleware(router)

	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// HSTS should be set when behind HTTPS proxy
	hsts := w.Header().Get("Strict-Transport-Security")
	assert.NotEmpty(t, hsts)
}

func TestSetupBasicMiddleware_NoHSTS_HTTP(t *testing.T) {
	router := chi.NewRouter()
	setupBasicMiddleware(router)

	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	// No TLS, no X-Forwarded-Proto
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// HSTS should NOT be set for plain HTTP
	assert.Empty(t, w.Header().Get("Strict-Transport-Security"))
}

// =============================================================================
// Edge Cases and Error Handling Tests
// =============================================================================

func TestSetupRateLimiting_OnlyRequestsPerMinuteSet(t *testing.T) {
	t.Setenv("RATE_LIMIT_ENABLED", "true")
	t.Setenv("RATE_LIMIT_REQUESTS_PER_MINUTE", "30")
	// RATE_LIMIT_BURST not set - should use default

	router := chi.NewRouter()
	setupRateLimiting(router, nil)

	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.150:12345"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSetupRateLimiting_OnlyBurstSet(t *testing.T) {
	t.Setenv("RATE_LIMIT_ENABLED", "true")
	t.Setenv("RATE_LIMIT_BURST", "5")
	// RATE_LIMIT_REQUESTS_PER_MINUTE not set - should use default

	router := chi.NewRouter()
	setupRateLimiting(router, nil)

	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.151:12345"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestParseAllowedOrigins_EnvNotSet(t *testing.T) {
	// When env is not set, os.Getenv returns empty string
	// This is handled by the empty check in parseAllowedOrigins
	t.Setenv("CORS_ALLOWED_ORIGINS", "")

	origins := parseAllowedOrigins()
	assert.Equal(t, []string{"*"}, origins)
}

// =============================================================================
// API Struct Tests
// =============================================================================

func TestAPI_EmptyRouter(t *testing.T) {
	api := &API{
		Router: chi.NewRouter(),
	}

	req := httptest.NewRequest("GET", "/anything", nil)
	w := httptest.NewRecorder()
	api.ServeHTTP(w, req)

	// Empty router returns 404
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAPI_NilServices(t *testing.T) {
	// API can be created with nil services for basic routing
	api := &API{
		Router:   chi.NewRouter(),
		Services: nil,
	}

	api.Router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	api.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// =============================================================================
// CSP Header Tests
// =============================================================================

func TestSetupBasicMiddleware_CSPHeader(t *testing.T) {
	router := chi.NewRouter()
	setupBasicMiddleware(router)

	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	csp := w.Header().Get("Content-Security-Policy")
	assert.NotEmpty(t, csp)
	assert.Contains(t, csp, "default-src 'self'")
	assert.Contains(t, csp, "frame-ancestors 'none'")
}

func TestSetupBasicMiddleware_ReferrerPolicy(t *testing.T) {
	router := chi.NewRouter()
	setupBasicMiddleware(router)

	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, "strict-origin-when-cross-origin", w.Header().Get("Referrer-Policy"))
}

func TestSetupBasicMiddleware_PermissionsPolicy(t *testing.T) {
	router := chi.NewRouter()
	setupBasicMiddleware(router)

	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	permPolicy := w.Header().Get("Permissions-Policy")
	assert.Contains(t, permPolicy, "geolocation=()")
	assert.Contains(t, permPolicy, "microphone=()")
	assert.Contains(t, permPolicy, "camera=()")
}
