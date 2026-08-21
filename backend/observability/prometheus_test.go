package observability

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type staticSSEStatsProvider struct {
	stats SSEStats
}

func (p staticSSEStatsProvider) SnapshotStats() SSEStats {
	return p.stats
}

// Deliberately NOT parallel: RegisterSSEStatsProvider installs a
// process-global provider that MetricsHandler reads on every scrape, so two
// of these tests overwrite each other's provider.
func TestRefreshSSEGaugesResetsDisconnectedTenants(t *testing.T) {
	RegisterSSEStatsProvider(staticSSEStatsProvider{
		stats: SSEStats{ClientsByTenant: map[int64]int{101: 2, 202: 1}},
	})
	refreshSSEGauges()

	RegisterSSEStatsProvider(staticSSEStatsProvider{
		stats: SSEStats{ClientsByTenant: map[int64]int{202: 3}},
	})
	refreshSSEGauges()

	assert.Equal(t, float64(0), gaugeValue(t, "101"))
	assert.Equal(t, float64(3), gaugeValue(t, "202"))
}

func TestMetricsBearerTokenFromEnvRequiresConfiguredToken(t *testing.T) {
	t.Parallel()

	token, err := MetricsBearerTokenFromEnv(func(string) string {
		return "  "
	})

	require.Error(t, err)
	assert.Empty(t, token)
}

func TestMetricsBearerTokenFromEnvReturnsTrimmedToken(t *testing.T) {
	t.Parallel()

	token, err := MetricsBearerTokenFromEnv(func(string) string {
		return " correct-token "
	})

	require.NoError(t, err)
	assert.Equal(t, "correct-token", token)
}

func TestMetricsAuthMiddlewareRejectsWrongToken(t *testing.T) {
	t.Parallel()

	handler := MetricsAuthMiddleware("correct-token")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called with the wrong token")
	}))
	request := httptest.NewRequest(http.MethodGet, "/internal/metrics", nil)
	request.Header.Set("Authorization", "Bearer wrong-token")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, request)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestMetricsAuthMiddlewareAllowsCorrectToken(t *testing.T) {
	t.Parallel()

	handler := MetricsAuthMiddleware("correct-token")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	request := httptest.NewRequest(http.MethodGet, "/internal/metrics", nil)
	request.Header.Set("Authorization", "Bearer correct-token")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, request)

	assert.Equal(t, http.StatusAccepted, rr.Code)
}

// Deliberately NOT parallel: RegisterSSEStatsProvider installs a
// process-global provider that MetricsHandler reads on every scrape, so two
// of these tests overwrite each other's provider.
func TestMetricsHandlerRefreshesSSEGaugesBeforeServing(t *testing.T) {
	RegisterSSEStatsProvider(staticSSEStatsProvider{
		stats: SSEStats{ClientsByTenant: map[int64]int{303: 5}},
	})
	request := httptest.NewRequest(http.MethodGet, "/internal/metrics", nil)

	rr := httptest.NewRecorder()
	MetricsHandler().ServeHTTP(rr, request)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `phoenix_sse_clients{tenant_id="303"} 5`)
}

func TestPrometheusRecordersUseStableLabels(t *testing.T) {
	t.Parallel()

	ObserveTenantRequest(404, "", http.MethodGet, "/api/students/{id}", http.StatusCreated, time.Millisecond, "commit")
	ObserveIoTRequest(0, http.MethodPost, "/api/iot/scan", http.StatusUnauthorized, time.Millisecond, "")
	RecordSSEConnection(404, "connected")
	RecordSSEBroadcast(0, "student_location_changed", "all", 2)

	assert.Equal(t, "unknown", StatusClass(0))
	assert.Equal(t, "2xx", StatusClass(http.StatusNoContent))
	assert.Equal(t, "unknown", sanitizeLabel(" "))
	assert.Equal(t, "auth_error", outcomeForStatus(http.StatusForbidden))
	assert.Equal(t, "validation_error", outcomeForStatus(http.StatusBadRequest))
	assert.Equal(t, "server_error", outcomeForStatus(http.StatusInternalServerError))
	assert.Equal(t, "success", outcomeForStatus(http.StatusOK))
}

func TestRoutePatternFallsBackToUnmatched(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodGet, "/not-mounted", nil)

	assert.Equal(t, "unmatched", RoutePattern(request))
}

func TestDBStatsCollectorEmitsProviderMetrics(t *testing.T) {
	t.Parallel()

	RegisterDBStatsProvider(fakeDBStatsProvider{stats: sql.DBStats{
		OpenConnections:   8,
		InUse:             3,
		Idle:              5,
		WaitCount:         13,
		WaitDuration:      2 * time.Second,
		MaxIdleClosed:     21,
		MaxLifetimeClosed: 34,
	}})
	ch := make(chan prometheus.Metric, 10)

	dbStatsCollector{}.Collect(ch)
	close(ch)

	var metricCount int
	for range ch {
		metricCount++
	}
	assert.Equal(t, 7, metricCount)
}

func TestDBStatsCollectorDescribesEveryEmittedMetric(t *testing.T) {
	t.Parallel()

	ch := make(chan *prometheus.Desc, 10)

	dbStatsCollector{}.Describe(ch)
	close(ch)

	var descCount int
	for range ch {
		descCount++
	}
	assert.Equal(t, 7, descCount)
}

func TestMain(m *testing.M) {
	code := m.Run()
	RegisterDBStatsProvider(nil)
	RegisterSSEStatsProvider(nil)
	os.Exit(code)
}

func gaugeValue(t *testing.T, tenant string) float64 {
	t.Helper()
	return testutil.ToFloat64(sseClients.WithLabelValues(tenant))
}
