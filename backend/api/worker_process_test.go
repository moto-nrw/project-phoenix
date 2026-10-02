package api

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type leaseProbeWorker struct {
	leading atomic.Bool
	started chan struct{}
	stopped chan struct{}
}

func (worker *leaseProbeWorker) Start()      { close(worker.started) }
func (worker *leaseProbeWorker) Stop()       { close(worker.stopped) }
func (worker *leaseProbeWorker) Ready() bool { return worker.leading.Load() }

func probe(t *testing.T, handler http.Handler, path, token string) (int, string) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder.Code, recorder.Body.String()
}

func TestWorkerProbesGateReadinessOnTheLease(t *testing.T) {
	t.Parallel()
	var leading atomic.Bool
	handler := workerProbeHandler(leading.Load, "metrics-token")

	code, body := probe(t, handler, "/ready", "")
	assert.Equal(t, http.StatusServiceUnavailable, code, "a standby is alive but not ready")
	assert.Equal(t, "standby", body)
	code, _ = probe(t, handler, "/health", "")
	assert.Equal(t, http.StatusOK, code)

	leading.Store(true)
	code, body = probe(t, handler, "/ready", "")
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "ready", body)

	code, _ = probe(t, handler, "/internal/metrics", "")
	assert.Equal(t, http.StatusUnauthorized, code)
	code, body = probe(t, handler, "/internal/metrics", "metrics-token")
	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, "phoenix_worker_lease_held")
}

func TestWorkerProbesNeverServeMetricsWithoutAToken(t *testing.T) {
	t.Parallel()
	handler := workerProbeHandler(func() bool { return true }, "")

	code, _ := probe(t, handler, "/internal/metrics", "")

	assert.Equal(t, http.StatusNotFound, code, "an empty token must not open the metrics")
}

func TestProcessHandlerServesProbesOnlyForTheStandaloneWorker(t *testing.T) {
	t.Parallel()
	apiHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	worker := &leaseProbeWorker{}

	for _, process := range []ServeProcess{ServeAPIWithWorker, ServeAPIOnly} {
		code, _ := probe(t, processHandler(process, apiHandler, worker, ""), "/ready", "")
		assert.Equal(t, http.StatusTeapot, code, "process %q serves the API", process)
	}
	code, _ := probe(t, processHandler(ServeWorkerOnly, apiHandler, worker, ""), "/ready", "")
	assert.Equal(t, http.StatusServiceUnavailable, code, "the standalone Worker serves only its probes")
}

func TestServeProcessValidation(t *testing.T) {
	t.Parallel()
	for _, process := range []ServeProcess{ServeAPIWithWorker, ServeAPIOnly, ServeWorkerOnly} {
		require.NoError(t, validateServeProcess(process))
	}

	require.ErrorContains(t, validateServeProcess("scheduler"), "unknown serve process")
}

func TestStandaloneWorkerServesItsProbesAndStopsTheWorker(t *testing.T) {
	t.Parallel()
	reserved, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = reserved.Close() })
	addr := reserved.Addr().String()
	worker := &leaseProbeWorker{started: make(chan struct{}), stopped: make(chan struct{})}
	runtime := &Runtime{
		server: &http.Server{Addr: addr, Handler: processHandler(ServeWorkerOnly, nil, worker, "")},
		worker: worker,
		logger: slog.New(slog.DiscardHandler),
		listen: func(_, _ string) (net.Listener, error) { return reserved, nil },
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runtime.Serve(ctx) }()

	select {
	case <-worker.started:
	case <-time.After(time.Second):
		t.Fatal("worker did not start")
	}
	readiness := func() int {
		response, err := http.Get("http://" + addr + "/ready") // #nosec G107 -- loopback test server
		require.NoError(t, err)
		_, _ = io.Copy(io.Discard, response.Body)
		require.NoError(t, response.Body.Close())
		return response.StatusCode
	}
	assert.Equal(t, http.StatusServiceUnavailable, readiness())
	worker.leading.Store(true)
	assert.Equal(t, http.StatusOK, readiness())

	cancel()
	require.NoError(t, <-done)
	select {
	case <-worker.stopped:
	default:
		t.Fatal("shutdown must stop the worker")
	}
}
