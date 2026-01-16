package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// parseAllowedOrigins Tests
// =============================================================================

func TestParseAllowedOrigins_Empty(t *testing.T) {
	// Clear env var
	os.Unsetenv("CORS_ALLOWED_ORIGINS")

	origins := parseAllowedOrigins()
	assert.Equal(t, []string{"*"}, origins)
}

func TestParseAllowedOrigins_SingleOrigin(t *testing.T) {
	os.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")
	defer os.Unsetenv("CORS_ALLOWED_ORIGINS")

	origins := parseAllowedOrigins()
	assert.Equal(t, []string{"http://localhost:3000"}, origins)
}

func TestParseAllowedOrigins_MultipleOrigins(t *testing.T) {
	os.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000, https://example.com, https://app.example.com")
	defer os.Unsetenv("CORS_ALLOWED_ORIGINS")

	origins := parseAllowedOrigins()
	assert.Equal(t, []string{"http://localhost:3000", "https://example.com", "https://app.example.com"}, origins)
}

func TestParseAllowedOrigins_TrimsWhitespace(t *testing.T) {
	os.Setenv("CORS_ALLOWED_ORIGINS", "  http://localhost:3000  ,  https://example.com  ")
	defer os.Unsetenv("CORS_ALLOWED_ORIGINS")

	origins := parseAllowedOrigins()
	assert.Equal(t, []string{"http://localhost:3000", "https://example.com"}, origins)
}

// =============================================================================
// parsePositiveInt Tests
// =============================================================================

func TestParsePositiveInt_Empty(t *testing.T) {
	os.Unsetenv("TEST_POSITIVE_INT")

	result := parsePositiveInt("TEST_POSITIVE_INT", 42)
	assert.Equal(t, 42, result)
}

func TestParsePositiveInt_ValidValue(t *testing.T) {
	os.Setenv("TEST_POSITIVE_INT", "100")
	defer os.Unsetenv("TEST_POSITIVE_INT")

	result := parsePositiveInt("TEST_POSITIVE_INT", 42)
	assert.Equal(t, 100, result)
}

func TestParsePositiveInt_ZeroReturnsDefault(t *testing.T) {
	os.Setenv("TEST_POSITIVE_INT", "0")
	defer os.Unsetenv("TEST_POSITIVE_INT")

	result := parsePositiveInt("TEST_POSITIVE_INT", 42)
	assert.Equal(t, 42, result)
}

func TestParsePositiveInt_NegativeReturnsDefault(t *testing.T) {
	os.Setenv("TEST_POSITIVE_INT", "-5")
	defer os.Unsetenv("TEST_POSITIVE_INT")

	result := parsePositiveInt("TEST_POSITIVE_INT", 42)
	assert.Equal(t, 42, result)
}

func TestParsePositiveInt_InvalidStringReturnsDefault(t *testing.T) {
	os.Setenv("TEST_POSITIVE_INT", "not_a_number")
	defer os.Unsetenv("TEST_POSITIVE_INT")

	result := parsePositiveInt("TEST_POSITIVE_INT", 42)
	assert.Equal(t, 42, result)
}

func TestParsePositiveInt_FloatReturnsDefault(t *testing.T) {
	os.Setenv("TEST_POSITIVE_INT", "10.5")
	defer os.Unsetenv("TEST_POSITIVE_INT")

	result := parsePositiveInt("TEST_POSITIVE_INT", 42)
	assert.Equal(t, 42, result)
}

// =============================================================================
// setupSecurityLogging Tests
// =============================================================================

func TestSetupSecurityLogging_Disabled(t *testing.T) {
	os.Unsetenv("SECURITY_LOGGING_ENABLED")
	router := chi.NewRouter()

	logger := setupSecurityLogging(router)
	assert.Nil(t, logger)
}

func TestSetupSecurityLogging_Enabled(t *testing.T) {
	os.Setenv("SECURITY_LOGGING_ENABLED", "true")
	defer os.Unsetenv("SECURITY_LOGGING_ENABLED")

	router := chi.NewRouter()
	logger := setupSecurityLogging(router)
	assert.NotNil(t, logger)
}

func TestSetupSecurityLogging_NotTrueValue(t *testing.T) {
	os.Setenv("SECURITY_LOGGING_ENABLED", "yes")
	defer os.Unsetenv("SECURITY_LOGGING_ENABLED")

	router := chi.NewRouter()
	logger := setupSecurityLogging(router)
	assert.Nil(t, logger)
}

// =============================================================================
// setupRateLimiting Tests
// =============================================================================

func TestSetupRateLimiting_Disabled(t *testing.T) {
	os.Unsetenv("RATE_LIMIT_ENABLED")
	router := chi.NewRouter()

	// Should not panic or add middleware when disabled
	setupRateLimiting(router, nil)
}

func TestSetupRateLimiting_Enabled(t *testing.T) {
	os.Setenv("RATE_LIMIT_ENABLED", "true")
	defer os.Unsetenv("RATE_LIMIT_ENABLED")

	router := chi.NewRouter()
	setupRateLimiting(router, nil)

	// Router should have middleware added (verifying no panic)
	require.NotNil(t, router)
}

func TestSetupRateLimiting_CustomValues(t *testing.T) {
	os.Setenv("RATE_LIMIT_ENABLED", "true")
	os.Setenv("RATE_LIMIT_REQUESTS_PER_MINUTE", "120")
	os.Setenv("RATE_LIMIT_BURST", "20")
	defer os.Unsetenv("RATE_LIMIT_ENABLED")
	defer os.Unsetenv("RATE_LIMIT_REQUESTS_PER_MINUTE")
	defer os.Unsetenv("RATE_LIMIT_BURST")

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

// =============================================================================
// setupCORS Tests
// =============================================================================

func TestSetupCORS(t *testing.T) {
	os.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")
	defer os.Unsetenv("CORS_ALLOWED_ORIGINS")

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
