package observability

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPMetricsMiddlewareRecordsRouteStatusAndLatency(t *testing.T) {
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
	recorder := &statusRecorder{
		ResponseWriter: httptest.NewRecorder(),
		status:         http.StatusOK,
	}

	_, ok := any(recorder).(http.Flusher)

	assert.True(t, ok)
}
