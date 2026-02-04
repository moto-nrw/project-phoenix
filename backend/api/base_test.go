package api

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// =============================================================================
// setupBasicMiddleware Tests
// =============================================================================

func TestSetupBasicMiddleware(t *testing.T) {
	router := chi.NewRouter()
	setupBasicMiddleware(router, slog.Default())

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
