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

// #3638: the Sentry environment is APP_ENV. With a DSN set, only the four
// reporting environments may start; test, dev and local need an empty DSN.
func TestValidateServeConfig_SentryDSNRejectsUnknownAppEnv(t *testing.T) {
	t.Parallel()
	for _, appEnv := range []string{"test", "dev", "local", "prod"} {
		t.Run(appEnv, func(t *testing.T) {
			t.Parallel()
			config := validServeConfig()
			config.AppEnv = appEnv
			config.SentryDSN = "https://example@sentry.io/123"
			config.SentryPyrePortalDSN = "https://kiosk@sentry.io/456"

			err := validateServeConfig(config)

			require.Error(t, err)
			assert.Contains(t, err.Error(), `APP_ENV="`+appEnv+`"`)
			assert.Contains(t, err.Error(), "production, staging, demo, development")
		})
	}
}

func TestValidateServeConfig_WithoutSentryDSNAllowsLocalAppEnv(t *testing.T) {
	t.Parallel()
	for _, appEnv := range []string{"dev", "local"} {
		config := validServeConfig()
		config.AppEnv = appEnv

		require.NoError(t, validateServeConfig(config), appEnv)
	}
}

func TestValidateServeConfig_SentryDSNAcceptsReportingAppEnv(t *testing.T) {
	t.Parallel()
	for _, appEnv := range []string{"production", "staging", "demo", "development"} {
		config := validServeConfig()
		config.AppEnv = appEnv
		config.SentryDSN = "https://example@sentry.io/123"
		config.SentryPyrePortalDSN = "https://kiosk@sentry.io/456"

		require.NoError(t, validateServeConfig(config), appEnv)
	}
}

func TestSentryClientOptions_UsesAppEnvAsEnvironment(t *testing.T) {
	t.Parallel()
	config := validServeConfig()
	config.AppEnv = "demo"
	config.SentryDSN = " https://example@sentry.io/123 "

	options := sentryClientOptions(config)

	assert.Equal(t, "demo", options.Environment)
	assert.Equal(t, "https://example@sentry.io/123", options.Dsn)
	assert.Equal(t, release, options.Release)
	require.NotNil(t, options.BeforeSend)
}

// #3645: a backend that reports to Sentry relays the kiosks' reports too, so
// it refuses to start without the pyreportal DSN.
func TestValidateServeConfig_SentryDSNRequiresPyrePortalDSN(t *testing.T) {
	t.Parallel()
	config := validServeConfig()
	config.AppEnv = "staging"
	config.SentryDSN = "https://example@sentry.io/123"

	err := validateServeConfig(config)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "SENTRY_PYREPORTAL_DSN")
}

func TestScrubSentryEvent_RemovesRequestDataAndSensitiveHeaders(t *testing.T) {
	t.Parallel()
	event := &sentry.Event{
		Request: &sentry.Request{
			Data: `{"notes":"person names and free text"}`,
			Headers: map[string]string{
				"Authorization":    "Basic replayable-caldav-credentials",
				"X-Staff-PIN":      "1234",
				"X-Staff-Auth-PIN": "personal-pin",
				"Accept":           "application/json",
			},
		},
	}

	scrubbed := scrubSentryEvent(event)

	require.NotNil(t, scrubbed.Request)
	assert.Empty(t, scrubbed.Request.Data)
	assert.Equal(t, "[filtered]", scrubbed.Request.Headers["Authorization"])
	assert.Equal(t, "[filtered]", scrubbed.Request.Headers["X-Staff-PIN"])
	assert.Equal(t, "[filtered]", scrubbed.Request.Headers["X-Staff-Auth-PIN"])
	assert.Equal(t, "application/json", scrubbed.Request.Headers["Accept"])
}

func TestScrubSentryEvent_RedactsFeedTokens(t *testing.T) {
	t.Parallel()
	const calendarToken = "supersecretcalendarcapability123"
	const requestToken = "supersecretrequestcapability456"
	event := &sentry.Event{
		Message:     "GET /public/calendar/" + calendarToken + " then /public/request-feed/" + requestToken + " failed",
		Transaction: "/public/request-feed/" + requestToken,
		Request: &sentry.Request{
			URL:         "https://api.example/public/request-feed/" + requestToken,
			QueryString: "",
		},
		Breadcrumbs: []*sentry.Breadcrumb{
			{
				Message: "request /public/calendar/" + calendarToken,
				Data:    map[string]any{"url": "https://api.example/public/request-feed/" + requestToken},
			},
		},
	}

	scrubbed := scrubSentryEvent(event)

	// The capability token must not survive anywhere the SDK captured the path.
	assert.NotContains(t, scrubbed.Message, calendarToken)
	assert.NotContains(t, scrubbed.Message, requestToken)
	assert.NotContains(t, scrubbed.Transaction, requestToken)
	require.NotNil(t, scrubbed.Request)
	assert.NotContains(t, scrubbed.Request.URL, requestToken)
	assert.Contains(t, scrubbed.Request.URL, "[REDACTED]")
	require.Len(t, scrubbed.Breadcrumbs, 1)
	assert.NotContains(t, scrubbed.Breadcrumbs[0].Message, calendarToken)
	if url, ok := scrubbed.Breadcrumbs[0].Data["url"].(string); ok {
		assert.NotContains(t, url, requestToken)
	}
}

// Issue #3640: background failures carry error texts, and a mail server's
// rejection names the recipient. No e-mail address may reach Sentry.
func TestScrubSentryEvent_RedactsEmailAddresses(t *testing.T) {
	t.Parallel()
	const address = "parent.name+ogs@example-school.de"
	event := &sentry.Event{
		Message:   "delivery to " + address + " failed",
		Exception: []sentry.Exception{{Value: "550 5.1.1 <" + address + ">: Recipient address rejected"}},
		Breadcrumbs: []*sentry.Breadcrumb{{
			Message: "email send attempt failed",
			Data:    map[string]any{"error": "rcpt " + address + " refused", "attempt": 2},
		}},
	}

	scrubbed := scrubSentryEvent(event)

	assert.NotContains(t, scrubbed.Message, address)
	require.Len(t, scrubbed.Exception, 1)
	assert.Equal(t, "550 5.1.1 <[email]>: Recipient address rejected", scrubbed.Exception[0].Value)
	require.Len(t, scrubbed.Breadcrumbs, 1)
	assert.Equal(t, "rcpt [email] refused", scrubbed.Breadcrumbs[0].Data["error"])
	assert.Equal(t, 2, scrubbed.Breadcrumbs[0].Data["attempt"])
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
		PublicAPIURL:        "http://localhost:8080",
		ParentsURL:          "http://parents.localhost:3000",
		PhoenixAuthPassword: "phoenix_auth_dev",
		DatabaseDSN:         "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable",
	}
}
