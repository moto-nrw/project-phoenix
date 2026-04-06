package api

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseAllowedOrigins tests the parseAllowedOrigins function
func TestParseAllowedOrigins(t *testing.T) {
	tests := []struct {
		name              string
		envValue          string
		expectedExact     []string
		expectedWildcards []string
	}{
		{
			name:              "empty env var returns wildcard",
			envValue:          "",
			expectedExact:     []string{"*"},
			expectedWildcards: nil,
		},
		{
			name:              "single origin",
			envValue:          "http://localhost:3000",
			expectedExact:     []string{"http://localhost:3000"},
			expectedWildcards: nil,
		},
		{
			name:              "multiple origins with spaces",
			envValue:          "http://localhost:3000, https://example.com, https://app.example.com",
			expectedExact:     []string{"http://localhost:3000", "https://example.com", "https://app.example.com"},
			expectedWildcards: nil,
		},
		{
			name:              "multiple origins without spaces",
			envValue:          "http://localhost:3000,https://example.com,https://app.example.com",
			expectedExact:     []string{"http://localhost:3000", "https://example.com", "https://app.example.com"},
			expectedWildcards: nil,
		},
		{
			name:              "origins with excessive whitespace",
			envValue:          "  http://localhost:3000  ,  https://example.com  ",
			expectedExact:     []string{"http://localhost:3000", "https://example.com"},
			expectedWildcards: nil,
		},
		{
			name:              "wildcard explicitly set",
			envValue:          "*",
			expectedExact:     []string{"*"},
			expectedWildcards: nil,
		},
		{
			name:              "wildcard subdomain pattern",
			envValue:          "http://localhost:3000, *.example.com",
			expectedExact:     []string{"http://localhost:3000"},
			expectedWildcards: []string{".example.com"},
		},
		{
			name:              "only blanks fall back to wildcard",
			envValue:          " ,  , ",
			expectedExact:     []string{"*"},
			expectedWildcards: nil,
		},
		{
			name:              "only wildcard patterns",
			envValue:          "*.example.com, *.test.local",
			expectedExact:     nil,
			expectedWildcards: []string{".example.com", ".test.local"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable for this test
			t.Setenv("CORS_ALLOWED_ORIGINS", tt.envValue)

			// Call function under test
			exact, wildcards := parseAllowedOrigins()

			// Assert results
			assert.Equal(t, tt.expectedExact, exact)
			assert.Equal(t, tt.expectedWildcards, wildcards)
		})
	}
}

// TestParsePositiveInt tests the parsePositiveInt function
func TestParsePositiveInt(t *testing.T) {
	tests := []struct {
		name         string
		envVar       string
		envValue     string
		defaultValue int
		expected     int
	}{
		{
			name:         "empty env var returns default",
			envVar:       "TEST_INT_EMPTY",
			envValue:     "",
			defaultValue: 60,
			expected:     60,
		},
		{
			name:         "valid positive int",
			envVar:       "TEST_INT_VALID",
			envValue:     "100",
			defaultValue: 60,
			expected:     100,
		},
		{
			name:         "zero returns default",
			envVar:       "TEST_INT_ZERO",
			envValue:     "0",
			defaultValue: 60,
			expected:     60,
		},
		{
			name:         "negative returns default",
			envVar:       "TEST_INT_NEGATIVE",
			envValue:     "-5",
			defaultValue: 60,
			expected:     60,
		},
		{
			name:         "non-numeric string returns default",
			envVar:       "TEST_INT_INVALID",
			envValue:     "invalid",
			defaultValue: 60,
			expected:     60,
		},
		{
			name:         "float string returns default",
			envVar:       "TEST_INT_FLOAT",
			envValue:     "12.5",
			defaultValue: 60,
			expected:     60,
		},
		{
			name:         "very large positive int",
			envVar:       "TEST_INT_LARGE",
			envValue:     "999999",
			defaultValue: 60,
			expected:     999999,
		},
		{
			name:         "whitespace returns default",
			envVar:       "TEST_INT_WHITESPACE",
			envValue:     "  ",
			defaultValue: 60,
			expected:     60,
		},
		{
			name:         "number with leading/trailing spaces returns default",
			envVar:       "TEST_INT_SPACES",
			envValue:     " 100 ",
			defaultValue: 60,
			expected:     60, // strconv.Atoi doesn't trim, so this fails
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable for this test
			if tt.envValue != "" {
				t.Setenv(tt.envVar, tt.envValue)
			}

			// Call function under test
			result := parsePositiveInt(tt.envVar, tt.defaultValue)

			// Assert result
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestParsePositiveInt_DifferentDefaults tests parsePositiveInt with various default values
func TestParsePositiveInt_DifferentDefaults(t *testing.T) {
	tests := []struct {
		name         string
		defaultValue int
	}{
		{"default 1", 1},
		{"default 10", 10},
		{"default 60", 60},
		{"default 100", 100},
		{"default 1000", 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Don't set any env var, so it uses default
			result := parsePositiveInt("NONEXISTENT_ENV_VAR", tt.defaultValue)
			assert.Equal(t, tt.defaultValue, result)
		})
	}
}

func TestInitializeAPIResources_WiresCaregiverServices(t *testing.T) {
	db, serviceFactory := testutil.SetupAPITest(t)
	defer func() { _ = db.Close() }()

	repoFactory := repositories.NewFactory(db)
	api := &API{
		Services: serviceFactory,
		Router:   chi.NewRouter(),
		db:       db,
		repos:    repoFactory,
	}

	initializeAPIResources(api, repoFactory, db, slog.Default())

	require.NotNil(t, api.Auth)
	require.NotNil(t, api.Operator)
	assert.Same(t, serviceFactory.CaregiverCapability, api.Auth.CaregiverCapabilityService)
}

func TestRegisterRoutesWithRateLimiting_MountsOperatorInvitationRoutes(t *testing.T) {
	db, serviceFactory := testutil.SetupAPITest(t)
	defer func() { _ = db.Close() }()

	repoFactory := repositories.NewFactory(db)
	api := &API{
		Services: serviceFactory,
		Router:   chi.NewRouter(),
		db:       db,
		repos:    repoFactory,
	}

	initializeAPIResources(api, repoFactory, db, slog.Default())

	previousEnabled := viper.Get("rate_limit_enabled")
	previousPerMinute := viper.Get("rate_limit_per_minute")
	viper.Set("rate_limit_enabled", true)
	viper.Set("rate_limit_per_minute", 5)
	t.Cleanup(func() {
		viper.Set("rate_limit_enabled", previousEnabled)
		viper.Set("rate_limit_per_minute", previousPerMinute)
	})

	api.registerRoutesWithRateLimiting()

	req := httptest.NewRequest(http.MethodPost, "/operator/auth/invitations/validate", nil)
	rr := httptest.NewRecorder()

	api.Router.ServeHTTP(rr, req)

	assert.NotEqual(t, http.StatusNotFound, rr.Code)
}
