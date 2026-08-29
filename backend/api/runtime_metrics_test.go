package api

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type flushRecorder struct {
	*httptest.ResponseRecorder
	flushed bool
}

func (r *flushRecorder) Flush() { r.flushed = true }

type fakeSSEClientCounter struct{ count int }

func (c fakeSSEClientCounter) GetClientCount() int { return c.count }

func TestHTTPMetricsMiddlewareRecordsRouteStatusAndLatency(t *testing.T) {
	t.Parallel()
	metrics := newHTTPMetrics()
	router := chi.NewRouter()
	router.Use(metrics.middleware)
	router.Get("/api/things/{thingID}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/things/abc", nil))
	active, routes := metrics.snapshot(10)
	assert.Equal(t, int64(0), active)
	require.Len(t, routes, 1)
	assert.Equal(t, http.MethodGet, routes[0].Method)
	assert.Equal(t, "/api/things/{thingID}", routes[0].Route)
	assert.Equal(t, "2xx", routes[0].StatusClass)
}

func TestHTTPMetricsMiddlewareSkipsHealthAndMetricsRoutes(t *testing.T) {
	t.Parallel()
	metrics := newHTTPMetrics()
	router := chi.NewRouter()
	router.Use(metrics.middleware)
	router.Get("/health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	router.Get("/internal/metrics", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/health", nil))
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/internal/metrics", nil))
	_, routes := metrics.snapshot(10)
	assert.Empty(t, routes)
}

func TestHTTPMetricsSnapshotSortsAndLimitsRoutes(t *testing.T) {
	t.Parallel()
	metrics := newHTTPMetrics()
	metrics.record(http.MethodGet, "/api/slow", http.StatusOK, time.Millisecond)
	metrics.record(http.MethodGet, "/api/error", http.StatusInternalServerError, time.Millisecond)
	_, routes := metrics.snapshot(1)
	require.Len(t, routes, 1)
	assert.Equal(t, "/api/error", routes[0].Route)
	assert.NotZero(t, routes[0].ErrorCount)
}

func TestStatusRecorderPreservesResponseWriterCapabilities(t *testing.T) {
	t.Parallel()
	base := &flushRecorder{ResponseRecorder: httptest.NewRecorder()}
	recorder := &statusRecorder{ResponseWriter: base, status: http.StatusOK}
	assert.Same(t, base, recorder.Unwrap())
	_, ok := any(recorder).(http.Flusher)
	assert.True(t, ok)
	recorder.Flush()
	assert.True(t, base.flushed)
}

func TestCapacityLoggerLogSnapshotUsesProviders(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{Level: slog.LevelInfo}))
	metrics := newHTTPMetrics()
	metrics.record(http.MethodGet, "/api/test", http.StatusOK, time.Millisecond)
	capacity := newCapacityLogger(
		func() dbCapacityStats { return dbCapacityStats{openConnections: 5, inUse: 2, idle: 3, waitCount: 7} },
		fakeSSEClientCounter{count: 4},
		metrics,
		logger,
	)
	capacity.LogSnapshot()
	logs := output.String()
	assert.Contains(t, logs, "capacity snapshot")
	assert.Contains(t, logs, "db_open_connections=5")
	assert.Contains(t, logs, "sse_clients=4")
	assert.Contains(t, logs, "http_active_requests=0")
}

func TestCapacityLoggerStartStopsOnContextCancel(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	capacity := newCapacityLogger(nil, nil, nil, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	capacity.interval = time.Hour
	done := make(chan struct{})
	go func() {
		defer close(done)
		capacity.Start(ctx)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("capacity logger did not stop after context cancellation")
	}
}

func TestMetricsAuthMiddlewareRejectsWrongToken(t *testing.T) {
	t.Parallel()
	handler := metricsAuthMiddleware("correct-token")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/internal/metrics", nil))

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestMetricsAuthMiddlewareAllowsConfiguredToken(t *testing.T) {
	t.Parallel()
	handler := metricsAuthMiddleware("correct-token")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/internal/metrics", nil)
	request.Header.Set("Authorization", "Bearer correct-token")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestMetricsHandlerServesPrometheusMetrics(t *testing.T) {
	t.Parallel()
	metrics := newHTTPMetrics()
	router := chi.NewRouter()
	router.Use(metrics.middleware)
	router.Get("/api/metrics-fixture", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/metrics-fixture", nil))
	recorder := httptest.NewRecorder()

	metricsHandler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/internal/metrics", nil))

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "phoenix_backend_http_requests_total")
}
