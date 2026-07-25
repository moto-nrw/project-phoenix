package cmd

import (
	"bytes"
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Command Registration Tests
// =============================================================================

func TestServeCmd_Metadata(t *testing.T) {
	assert.Equal(t, "serve", serveCmd.Use)
	assert.Contains(t, serveCmd.Short, "start http server")
	assert.Contains(t, serveCmd.Long, "http server")
	assert.NotNil(t, serveCmd.Run)
}

func TestServeCmd_IsRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range RootCmd.Commands() {
		if cmd.Use == "serve" {
			found = true
			break
		}
	}
	assert.True(t, found, "serveCmd should be registered on RootCmd")
}

func TestServeCmd_UsageOutput(t *testing.T) {
	buf := new(bytes.Buffer)
	serveCmd.SetOut(buf)
	serveCmd.SetErr(buf)

	err := serveCmd.Usage()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "serve")
}

// =============================================================================
// Serve config validation tests
// =============================================================================

func TestValidateServeConfig_ValidConfigPasses(t *testing.T) {
	resetServeConfig(t)
	setValidServeConfig()

	require.NoError(t, validateServeConfig())
}

func TestValidateServeConfig_MissingRequiredConfigFails(t *testing.T) {
	resetServeConfig(t)
	setValidServeConfig()
	viper.Set("auth_jwt_secret", "")

	err := validateServeConfig()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "AUTH_JWT_SECRET")
}

func TestValidateServeConfig_MissingEmailWebhookSigningSecretFails(t *testing.T) {
	resetServeConfig(t)
	setValidServeConfig()
	viper.Set("email_webhook_signing_secret", "")

	err := validateServeConfig()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "EMAIL_WEBHOOK_SIGNING_SECRET")
}

func TestValidateServeConfig_MessageIDDomainIsRequiredAndValidated(t *testing.T) {
	for _, domain := range []string{"", "mail_domain.example", "-mail.example", "mail.example:25"} {
		t.Run(domain, func(t *testing.T) {
			resetServeConfig(t)
			setValidServeConfig()
			viper.Set("email_message_id_domain", domain)

			err := validateServeConfig()

			require.Error(t, err)
			assert.Contains(t, err.Error(), "EMAIL_MESSAGE_ID_DOMAIN")
		})
	}
}

func TestValidateServeConfig_MissingDatabaseDSNFails(t *testing.T) {
	resetServeConfig(t)
	setValidServeConfig()
	viper.Set("db_dsn", "")

	err := validateServeConfig()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "DB_DSN")
}

func TestValidateServeConfig_TestEnvAllowsExplicitTestDSN(t *testing.T) {
	resetServeConfig(t)
	setValidServeConfig()
	viper.Set("app_env", "test")
	viper.Set("db_dsn", "")
	viper.Set("test_db_dsn", "postgres://postgres:postgres@localhost:5433/phoenix_test?sslmode=disable")

	require.NoError(t, validateServeConfig())
}

func TestValidateServeConfig_RejectsRandomJWTSecret(t *testing.T) {
	resetServeConfig(t)
	setValidServeConfig()
	viper.Set("auth_jwt_secret", "random")

	err := validateServeConfig()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "AUTH_JWT_SECRET=random")
}

func TestValidateServeConfig_SentryDSNRequiresEnvironment(t *testing.T) {
	resetServeConfig(t)
	setValidServeConfig()
	viper.Set("sentry_dsn", "https://example@sentry.io/123")

	err := validateServeConfig()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "SENTRY_ENVIRONMENT")
}

func TestValidateServeConfig_SentryEnvironmentPasses(t *testing.T) {
	resetServeConfig(t)
	setValidServeConfig()
	viper.Set("sentry_dsn", "https://example@sentry.io/123")
	viper.Set("sentry_environment", "staging")

	require.NoError(t, validateServeConfig())
}

func TestScrubSentryEvent_RemovesRequestDataAndSensitiveHeaders(t *testing.T) {
	event := &sentry.Event{
		Request: &sentry.Request{
			Data: `{"notes":"person names and free text"}`,
			Headers: map[string]string{
				"X-Staff-PIN": "1234",
				"Accept":      "application/json",
			},
		},
	}

	scrubbed := scrubSentryEvent(event)

	require.NotNil(t, scrubbed.Request)
	assert.Empty(t, scrubbed.Request.Data)
	assert.Equal(t, "[filtered]", scrubbed.Request.Headers["X-Staff-PIN"])
	assert.Equal(t, "application/json", scrubbed.Request.Headers["Accept"])
}

func TestScrubSentryEvent_RedactsCalendarFeedToken(t *testing.T) {
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

func resetServeConfig(t *testing.T) {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)
}

func setValidServeConfig() {
	viper.Set("port", "8080")
	viper.Set("app_env", "development")
	viper.Set("log_textlogging", "false")
	viper.Set("auth_jwt_secret", "test-jwt-secret-for-unit-tests-minimum-32-chars")
	viper.Set("auth_jwt_expiry", "15m")
	viper.Set("auth_jwt_refresh_expiry", "168h")
	viper.Set("frontend_url", "http://localhost:3000")
	viper.Set("parents_url", "http://parents.localhost:3000")
	viper.Set("phoenix_auth_password", "phoenix_auth_dev")
	viper.Set("email_webhook_signing_secret", "webhook-test-secret")
	viper.Set("email_message_id_domain", "mail.example.test")
	viper.Set("db_dsn", "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable")
}
