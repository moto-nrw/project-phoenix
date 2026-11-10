package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Each child owns its process configuration and metric registry. Parallel API
// tests must not change Viper/environment globals or reset shared counters.
func TestRateLimitConfigurationAndRejectionMetrics(t *testing.T) {
	t.Parallel()
	if os.Getenv("PHX_RATE_LIMIT_EXPECT") == "" {
		testName := t.Name()
		for _, tc := range []struct {
			value   string
			enabled bool
		}{{"1", true}, {"true", true}, {"0", false}, {"false", false}} {
			t.Run(tc.value, func(t *testing.T) {
				t.Parallel()
				output, code, err := testpkg.RunTestBinary(testName, []string{"PHX_RATE_LIMIT_EXPECT=" + strconv.FormatBool(tc.enabled), "RATE_LIMIT_ENABLED=" + tc.value,
					"RATE_LIMIT_REQUESTS_PER_MINUTE=1", "RATE_LIMIT_BURST=1", "RATE_LIMIT_AUTH_REQUESTS_PER_MINUTE=1", "SECURITY_LOGGING_ENABLED=false", "APP_ENV=development"})
				require.NoError(t, err, "%s", output)
				require.Zero(t, code, "%s", output)
			})
		}
		return
	}
	viper.AutomaticEnv()
	enabled := os.Getenv("PHX_RATE_LIMIT_EXPECT") == "true"
	cfg := CurrentServiceConfig()
	require.Equal(t, enabled, cfg.RateLimitEnabled, "password-reset configuration")
	accepted := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	general := chi.NewRouter()
	setupRateLimiting(general, nil, cfg.RateLimitEnabled, false)
	general.Post("/probe", accepted)
	auth := authRateLimitersFromEnv(cfg.RateLimitEnabled)
	webhook := emailWebhookRateLimit(cfg.RateLimitEnabled)
	handlers := map[string]struct {
		handler  http.Handler
		requests int
	}{"write": {general, 3}}
	if !enabled {
		assert.Nil(t, auth.auth)
		assert.Nil(t, auth.emailConfirm)
		assert.Nil(t, auth.invitation)
		assert.Nil(t, webhook)
	} else {
		require.NotNil(t, auth.auth)
		require.NotNil(t, auth.emailConfirm)
		require.NotNil(t, auth.invitation)
		require.NotNil(t, webhook)
		handlers["auth"] = struct {
			handler  http.Handler
			requests int
		}{auth.auth.Middleware()(accepted), 20}
		handlers["email-confirm"] = struct {
			handler  http.Handler
			requests int
		}{auth.emailConfirm.Middleware()(accepted), 20}
		handlers["invitation"] = struct {
			handler  http.Handler
			requests int
		}{auth.invitation.Middleware()(accepted), 20}
		handlers["webhook"] = struct {
			handler  http.Handler
			requests int
		}{webhook(accepted), 2 * emailWebhookBurst}
	}
	for bucket, probe := range handlers {
		before := rejectionMetric(t, bucket)
		rejected := 0
		for range probe.requests {
			request := httptest.NewRequest(http.MethodPost, "/probe", nil)
			request.RemoteAddr = "192.0.2.1:1234"
			response := httptest.NewRecorder()
			probe.handler.ServeHTTP(response, request)
			if response.Code == http.StatusTooManyRequests {
				rejected++
			} else {
				require.Equal(t, http.StatusNoContent, response.Code)
			}
		}
		if enabled {
			assert.Positive(t, rejected, "%s must enforce the configured flag", bucket)
		} else {
			assert.Zero(t, rejected)
		}
		assert.Equal(t, float64(rejected), rejectionMetric(t, bucket)-before, "%s must count exactly the refusals in its own bucket", bucket)
	}
}

func rejectionMetric(t *testing.T, bucket string) float64 {
	t.Helper()
	response := httptest.NewRecorder()
	metricsHandler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/internal/metrics", nil))
	require.Equal(t, http.StatusOK, response.Code)
	metric := fmt.Sprintf("phoenix_rate_limit_rejections_total{bucket=%q}", bucket)
	for _, line := range strings.Split(response.Body.String(), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == metric {
			value, err := strconv.ParseFloat(fields[1], 64)
			require.NoError(t, err)
			return value
		}
	}
	return 0
}
