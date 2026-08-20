package observability

import (
	"bytes"
	"context"
	"database/sql"
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

func (r *flushRecorder) Flush() {
	r.flushed = true
}

type fakeDBStatsProvider struct {
	stats sql.DBStats
}

func (p fakeDBStatsProvider) Stats() sql.DBStats {
	return p.stats
}

type fakeSSEClientCounter struct {
	count int
}

func (c fakeSSEClientCounter) GetClientCount() int {
	return c.count
}

func TestHTTPMetricsMiddlewareRecordsRouteStatusAndLatency(t *testing.T) {
	t.Parallel()

	metrics := NewHTTPMetrics()
	router := chi.NewRouter()
	router.Use(metrics.Middleware)
	router.Get("/api/things/{thingID}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/things/abc", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	active, routes := metrics.Snapshot(10)
	assert.Equal(t, int64(0), active)
	require.Len(t, routes, 1)
	assert.Equal(t, http.MethodGet, routes[0].Method)
	assert.Equal(t, "/api/things/{thingID}", routes[0].Route)
	assert.Equal(t, "2xx", routes[0].StatusClass)
	assert.NotZero(t, routes[0].Count)
}

func TestStatusRecorderPreservesFlusherForSSE(t *testing.T) {
	t.Parallel()

	recorder := &statusRecorder{
		ResponseWriter: httptest.NewRecorder(),
		status:         http.StatusOK,
	}

	_, ok := any(recorder).(http.Flusher)

	assert.True(t, ok)
}

func TestHTTPMetricsMiddlewareSkipsHealthAndMetricsRoutes(t *testing.T) {
	t.Parallel()

	metrics := NewHTTPMetrics()
	router := chi.NewRouter()
	router.Use(metrics.Middleware)
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	router.Get("/internal/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/health", nil))
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/internal/metrics", nil))

	active, routes := metrics.Snapshot(10)
	assert.Equal(t, int64(0), active)
	assert.Empty(t, routes)
}

func TestHTTPMetricsSnapshotSortsAndLimitsRoutes(t *testing.T) {
	t.Parallel()

	metrics := NewHTTPMetrics()
	router := chi.NewRouter()
	router.Use(metrics.Middleware)
	router.Get("/api/slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})
	router.Get("/api/error", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/slow", nil))
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/error", nil))

	_, routes := metrics.Snapshot(1)
	require.Len(t, routes, 1)
	assert.Equal(t, "/api/error", routes[0].Route)
	assert.NotZero(t, routes[0].ErrorCount)
}

func TestStatusRecorderUnwrapAndFlush(t *testing.T) {
	t.Parallel()

	base := &flushRecorder{ResponseRecorder: httptest.NewRecorder()}
	recorder := &statusRecorder{ResponseWriter: base, status: http.StatusOK}

	assert.Same(t, base, recorder.Unwrap())
	recorder.Flush()

	assert.True(t, base.flushed)
}

func TestCapacityLoggerLogSnapshotUsesProviders(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	metrics := NewHTTPMetrics()
	metrics.record(httptest.NewRequest(http.MethodGet, "/api/test", nil), http.StatusOK, time.Millisecond)
	capacityLogger := NewCapacityLogger(
		fakeDBStatsProvider{stats: sql.DBStats{OpenConnections: 5, InUse: 2, Idle: 3, WaitCount: 7}},
		fakeSSEClientCounter{count: 4},
		metrics,
		logger,
	)

	capacityLogger.LogSnapshot()

	logs := buf.String()
	assert.Contains(t, logs, "capacity snapshot")
	assert.Contains(t, logs, "db_open_connections=5")
	assert.Contains(t, logs, "sse_clients=4")
	assert.Contains(t, logs, "http_active_requests=0")
}

func TestCapacityLoggerStartStopsOnContextCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	capacityLogger := NewCapacityLogger(nil, nil, nil, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	capacityLogger.interval = time.Hour
	done := make(chan struct{})

	go func() {
		defer close(done)
		capacityLogger.Start(ctx)
	}()
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("capacity logger did not stop after context cancellation")
	}
}
