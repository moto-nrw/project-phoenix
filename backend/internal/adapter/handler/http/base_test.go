package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// parseAllowedOrigins Tests
// =============================================================================

func TestParseAllowedOrigins_Empty(t *testing.T) {
	viper.Reset()

	origins, err := parseAllowedOrigins()
	require.Error(t, err)
	assert.Empty(t, origins)
}

func TestParseAllowedOrigins_SingleOrigin(t *testing.T) {
	viper.Reset()
	viper.Set("cors_allowed_origins", "http://localhost:3000")

	origins, err := parseAllowedOrigins()
	require.NoError(t, err)
	assert.Equal(t, []string{"http://localhost:3000"}, origins)
}

func TestParseAllowedOrigins_MultipleOrigins(t *testing.T) {
	viper.Reset()
	viper.Set("cors_allowed_origins", "http://localhost:3000, https://example.com, https://app.example.com")

	origins, err := parseAllowedOrigins()
	require.NoError(t, err)
	assert.Equal(t, []string{"http://localhost:3000", "https://example.com", "https://app.example.com"}, origins)
}

func TestParseAllowedOrigins_TrimsWhitespace(t *testing.T) {
	viper.Reset()
	viper.Set("cors_allowed_origins", "  http://localhost:3000  ,  https://example.com  ")

	origins, err := parseAllowedOrigins()
	require.NoError(t, err)
	assert.Equal(t, []string{"http://localhost:3000", "https://example.com"}, origins)
}

// =============================================================================
// parsePositiveInt Tests
// =============================================================================

func TestParseRequiredPositiveInt_Empty(t *testing.T) {
	viper.Reset()

	_, err := parseRequiredPositiveInt("TEST_POSITIVE_INT")
	require.Error(t, err)
}

func TestParseRequiredPositiveInt_ValidValue(t *testing.T) {
	viper.Reset()
	viper.Set("TEST_POSITIVE_INT", "100")

	result, err := parseRequiredPositiveInt("TEST_POSITIVE_INT")
	require.NoError(t, err)
	assert.Equal(t, 100, result)
}

func TestParseRequiredPositiveInt_ZeroReturnsError(t *testing.T) {
	viper.Reset()
	viper.Set("TEST_POSITIVE_INT", "0")

	_, err := parseRequiredPositiveInt("TEST_POSITIVE_INT")
	require.Error(t, err)
}

func TestParseRequiredPositiveInt_NegativeReturnsError(t *testing.T) {
	viper.Reset()
	viper.Set("TEST_POSITIVE_INT", "-5")

	_, err := parseRequiredPositiveInt("TEST_POSITIVE_INT")
	require.Error(t, err)
}

func TestParseRequiredPositiveInt_InvalidStringReturnsError(t *testing.T) {
	viper.Reset()
	viper.Set("TEST_POSITIVE_INT", "not_a_number")

	_, err := parseRequiredPositiveInt("TEST_POSITIVE_INT")
	require.Error(t, err)
}

func TestParseRequiredPositiveInt_FloatReturnsError(t *testing.T) {
	viper.Reset()
	viper.Set("TEST_POSITIVE_INT", "10.5")

	_, err := parseRequiredPositiveInt("TEST_POSITIVE_INT")
	require.Error(t, err)
}

// =============================================================================
// setupSecurityLogging Tests
// =============================================================================

func TestSetupSecurityLogging_Disabled(t *testing.T) {
	viper.Reset()
	router := chi.NewRouter()

	logger := setupSecurityLogging(router)
	assert.Nil(t, logger)
}

func TestSetupSecurityLogging_Enabled(t *testing.T) {
	viper.Reset()
	viper.Set("SECURITY_LOGGING_ENABLED", "true")

	router := chi.NewRouter()
	logger := setupSecurityLogging(router)
	assert.NotNil(t, logger)
}

func TestSetupSecurityLogging_NotTrueValue(t *testing.T) {
	viper.Reset()
	viper.Set("SECURITY_LOGGING_ENABLED", "yes")

	router := chi.NewRouter()
	logger := setupSecurityLogging(router)
	assert.Nil(t, logger)
}

// =============================================================================
// setupRateLimiting Tests
// =============================================================================

func TestSetupRateLimiting_Disabled(t *testing.T) {
	viper.Reset()
	router := chi.NewRouter()

	// Should not panic or add middleware when disabled
	err := setupRateLimiting(router, nil)
	require.NoError(t, err)
}

func TestSetupRateLimiting_Enabled(t *testing.T) {
	viper.Reset()
	viper.Set("RATE_LIMIT_ENABLED", "true")

	router := chi.NewRouter()
	err := setupRateLimiting(router, nil)

	require.Error(t, err)
	require.NotNil(t, router)
}

func TestSetupRateLimiting_CustomValues(t *testing.T) {
	viper.Reset()
	viper.Set("RATE_LIMIT_ENABLED", "true")
	viper.Set("RATE_LIMIT_REQUESTS_PER_MINUTE", "120")
	viper.Set("RATE_LIMIT_BURST", "20")

	router := chi.NewRouter()
	err := setupRateLimiting(router, nil)

	// Should complete without error
	require.NoError(t, err)
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
	viper.Reset()
	viper.Set("cors_allowed_origins", "http://localhost:3000")

	router := chi.NewRouter()
	err := setupCORS(router)
	require.NoError(t, err)

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
