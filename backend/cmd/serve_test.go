package cmd

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
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

// One Turnstile widget protects every school's public enrollment form, so
// the server refuses to start without both of its keys.
func TestValidateServeConfig_MissingTurnstileKeysFail(t *testing.T) {
	t.Parallel()
	for _, missing := range []string{"TURNSTILE_SITE_KEY", "TURNSTILE_SECRET_KEY"} {
		config := validServeConfig()
		if missing == "TURNSTILE_SITE_KEY" {
			config.TurnstileSiteKey = " "
		} else {
			config.TurnstileSecretKey = ""
		}

		err := validateServeConfig(config)

		require.Error(t, err, missing)
		assert.Contains(t, err.Error(), missing)
	}
}

// The health information of the holiday registrations is sealed with
// HEALTH_DATA_KEYS (#4722): without a usable key ring the server refuses to
// start instead of failing at the first registration.
func TestValidateServeConfig_MissingOrUnusableHealthDataKeysFail(t *testing.T) {
	t.Parallel()
	missing := validServeConfig()
	missing.HealthDataKeys = " "
	err := validateServeConfig(missing)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HEALTH_DATA_KEYS")

	unusable := validServeConfig()
	unusable.HealthDataKeys = "k1:" + base64.StdEncoding.EncodeToString([]byte("too short"))
	err = validateServeConfig(unusable)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HEALTH_DATA_KEYS")
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
		// Cloudflare's documented always-passing Turnstile test keys.
		TurnstileSiteKey:   "1x00000000000000000000AA",
		TurnstileSecretKey: "1x0000000000000000000000000000000AA",
		HealthDataKeys:     testHealthDataKeys(),
		BankDataKeys:       testHealthDataKeys(),
	}
}

// testHealthDataKeys is a key ring with one random key (#4722).
func testHealthDataKeys() string {
	key := make([]byte, 32)
	if err := securityruntime.FillRandom(key); err != nil {
		panic(err)
	}
	return "test:" + base64.StdEncoding.EncodeToString(key)
}

func TestScrubSentryEventRedactsContractSigningCredentials(t *testing.T) {
	t.Parallel()
	path := "/public/care-contract-signing/42.sentry-signing-credential/sign"
	page := "https://parents.example/vertrag-unterschreiben/42.sentry-signing-credential"
	event := &sentry.Event{
		Message: "request " + path, Transaction: path,
		Request:     &sentry.Request{URL: "https://api.example" + path, Headers: map[string]string{"Referer": page, "Content-Type": "application/json"}},
		Exception:   []sentry.Exception{{Value: "failed " + path}},
		Breadcrumbs: []*sentry.Breadcrumb{nil, {Message: page, Data: map[string]any{"url": path, "status": 500}}},
	}
	scrubbed := scrubSentryEvent(event)
	data, err := json.Marshal(scrubbed)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "sentry-signing-credential")
	assert.Equal(t, "https://api.example/public/care-contract-signing/[REDACTED]/sign", scrubbed.Request.URL)
	assert.Equal(t, "application/json", scrubbed.Request.Headers["Content-Type"])
	assert.Equal(t, 500, scrubbed.Breadcrumbs[1].Data["status"])
}

// signingToken is the secret part of a care contract's one-time link
// (#4679); it must reach Sentry in no field.
const signingToken = "geheimes-signier-token-0123456789"

func signingPath() string { return "/public/care-contract-signing/7." + signingToken + "/sign" }

// TestScrubSentryEvent_RedactsSigningTokensInEveryField: one case per field
// that carries text, each red when that field is sent unscrubbed.
func TestScrubSentryEvent_RedactsSigningTokensInEveryField(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		event *sentry.Event
		field func(*sentry.Event) string
	}{
		"message": {
			event: &sentry.Event{Message: "POST " + signingPath() + " failed"},
			field: func(e *sentry.Event) string { return e.Message },
		},
		"exception value": {
			event: &sentry.Event{Exception: []sentry.Exception{{Value: "request to " + signingPath() + " timed out"}}},
			field: func(e *sentry.Event) string { return e.Exception[0].Value },
		},
		"breadcrumb message": {
			event: &sentry.Event{Breadcrumbs: []*sentry.Breadcrumb{{Message: "GET " + signingPath()}}},
			field: func(e *sentry.Event) string { return e.Breadcrumbs[0].Message },
		},
		"breadcrumb data": {
			event: &sentry.Event{Breadcrumbs: []*sentry.Breadcrumb{{Data: map[string]any{"url": "https://api.example" + signingPath()}}}},
			field: func(e *sentry.Event) string { return e.Breadcrumbs[0].Data["url"].(string) },
		},
		"request url": {
			event: &sentry.Event{Request: &sentry.Request{URL: "https://api.example" + signingPath()}},
			field: func(e *sentry.Event) string { return e.Request.URL },
		},
		"request query": {
			event: &sentry.Event{Request: &sentry.Request{QueryString: "back=" + signingPath()}},
			field: func(e *sentry.Event) string { return e.Request.QueryString },
		},
		"contexts": {
			event: &sentry.Event{Contexts: map[string]sentry.Context{"request": {
				"url": "https://api.example" + signingPath(), "nested": map[string]any{"path": []any{signingPath()}},
			}}},
			field: func(e *sentry.Event) string {
				encoded, err := json.Marshal(e.Contexts)
				require.NoError(t, err)
				return string(encoded)
			},
		},
		"tags": {
			event: &sentry.Event{Tags: map[string]string{"url": "https://api.example" + signingPath()}},
			field: func(e *sentry.Event) string { return e.Tags["url"] },
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			value := tc.field(scrubSentryEvent(tc.event))
			assert.NotContains(t, value, signingToken)
			assert.Contains(t, value, "[REDACTED]")
		})
	}
}

func TestValidateServeConfig_BankRingRequiredWithoutFallback(t *testing.T) {
	t.Parallel()
	for _, invalid := range []string{"", " ", "malformed", "bank:c2hvcnQ="} {
		config := validServeConfig()
		config.BankDataLegacyKeys = config.HealthDataKeys
		config.BankDataKeys = invalid
		err := validateServeConfig(config)
		require.Error(t, err)
		require.Contains(t, err.Error(), "BANK_DATA_KEYS")
		require.NotContains(t, err.Error(), config.HealthDataKeys)
	}
}
