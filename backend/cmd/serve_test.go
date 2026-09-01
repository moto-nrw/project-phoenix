package cmd

import (
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Command Registration Tests
// =============================================================================

func TestServeCmd_Metadata(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "serve", serveCmd.Use)
	assert.Contains(t, serveCmd.Short, "start http server")
	assert.Contains(t, serveCmd.Long, "http server")
	assert.NotNil(t, serveCmd.RunE)
}

// =============================================================================
// Serve config validation tests
// =============================================================================

func TestValidateServeConfig_ValidConfigPasses(t *testing.T) {
	t.Parallel()
	require.NoError(t, validateServeConfig(validServeConfig()))
}

func TestValidateServeConfig_MissingRequiredConfigFails(t *testing.T) {
	t.Parallel()
	config := validServeConfig()
	config.JWTSecret = ""

	err := validateServeConfig(config)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "AUTH_JWT_SECRET")
}

func TestValidateServeConfig_MissingDatabaseDSNFails(t *testing.T) {
	t.Parallel()
	config := validServeConfig()
	config.DatabaseDSN = ""

	err := validateServeConfig(config)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "DB_DSN")
}

func TestValidateServeConfig_TestEnvAllowsExplicitTestDSN(t *testing.T) {
	t.Parallel()
	config := validServeConfig()
	config.AppEnv = "test"
	config.DatabaseDSN = ""
	config.TestDatabaseDSN = "postgres://postgres:postgres@localhost:5433/phoenix_test?sslmode=disable"

	require.NoError(t, validateServeConfig(config))
}

func TestValidateServeConfig_RejectsRandomJWTSecret(t *testing.T) {
	t.Parallel()
	config := validServeConfig()
	config.JWTSecret = "random"

	err := validateServeConfig(config)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "AUTH_JWT_SECRET=random")
}

func TestValidateServeConfig_SentryDSNRequiresEnvironment(t *testing.T) {
	t.Parallel()
	config := validServeConfig()
	config.SentryDSN = "https://example@sentry.io/123"

	err := validateServeConfig(config)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "SENTRY_ENVIRONMENT")
}

func TestValidateServeConfig_SentryEnvironmentPasses(t *testing.T) {
	t.Parallel()
	config := validServeConfig()
	config.SentryDSN = "https://example@sentry.io/123"
	config.SentryEnvironment = "staging"

	require.NoError(t, validateServeConfig(config))
}

func TestScrubSentryEvent_RemovesRequestDataAndSensitiveHeaders(t *testing.T) {
	t.Parallel()
	event := &sentry.Event{
		Request: &sentry.Request{
			Data: `{"notes":"person names and free text"}`,
			Headers: map[string]string{
				"X-Staff-PIN":      "1234",
				"X-Staff-Auth-Pin": "5678",
				"Accept":           "application/json",
			},
		},
	}

	scrubbed := scrubSentryEvent(event)

	require.NotNil(t, scrubbed.Request)
	assert.Empty(t, scrubbed.Request.Data)
	assert.Equal(t, "[filtered]", scrubbed.Request.Headers["X-Staff-PIN"])
	assert.Equal(t, "[filtered]", scrubbed.Request.Headers["X-Staff-Auth-Pin"])
	assert.Equal(t, "application/json", scrubbed.Request.Headers["Accept"])
}

func TestScrubSentryEvent_RedactsCalendarFeedToken(t *testing.T) {
	t.Parallel()
	const token = "supersecretcapabilitytoken123456"
	event := &sentry.Event{
		Message:     "GET /public/calendar/" + token + " failed",
		Transaction: "/public/calendar/" + token,
		Request: &sentry.Request{
			URL:         "https://api.example/public/calendar/" + token,
			QueryString: "",
		},
		Breadcrumbs: []*sentry.Breadcrumb{
			{
				Message: "request /public/calendar/" + token,
				Data:    map[string]any{"url": "https://api.example/public/calendar/" + token},
			},
		},
	}

	scrubbed := scrubSentryEvent(event)

	// The capability token must not survive anywhere the SDK captured the path.
	assert.NotContains(t, scrubbed.Message, token)
	assert.NotContains(t, scrubbed.Transaction, token)
	require.NotNil(t, scrubbed.Request)
	assert.NotContains(t, scrubbed.Request.URL, token)
	assert.Contains(t, scrubbed.Request.URL, "[REDACTED]")
	require.Len(t, scrubbed.Breadcrumbs, 1)
	assert.NotContains(t, scrubbed.Breadcrumbs[0].Message, token)
	if url, ok := scrubbed.Breadcrumbs[0].Data["url"].(string); ok {
		assert.NotContains(t, url, token)
	}
}

func validServeConfig() serveConfig {
	return serveConfig{
		Port:                "8080",
		AppEnv:              "development",
		LogTextLoggingRaw:   "false",
		JWTSecret:           "test-jwt-secret-for-unit-tests-minimum-32-chars",
		JWTExpiry:           "15m",
		JWTRefreshExpiry:    "168h",
		FrontendURL:         "http://localhost:3000",
		ParentsURL:          "http://parents.localhost:3000",
		PhoenixAuthPassword: "phoenix_auth_dev",
		DatabaseDSN:         "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable",
	}
}
