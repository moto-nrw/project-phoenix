package api

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type blockingWorker struct {
	release <-chan struct{}
}

func (worker blockingWorker) Start() {}

func (worker blockingWorker) Stop() {
	<-worker.release
}

type recordingWorker struct{ started chan struct{} }

func (worker recordingWorker) Start() { close(worker.started) }

func (recordingWorker) Stop() {}

type readinessWorker struct {
	url     string
	started chan error
}

func (worker readinessWorker) Start() {
	client := http.Client{Timeout: 200 * time.Millisecond}
	response, err := client.Get(worker.url) // #nosec G107 -- loopback test server
	if err == nil {
		err = response.Body.Close()
	}
	worker.started <- err
}

func (readinessWorker) Stop() {}

func checkRuntimeRejectsMissingDependencies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ctx  context.Context
		deps ServeConfig
		run  func(*Runtime) error
		want string
	}{
		{
			name: "logger",
			ctx:  context.Background(),
			deps: ServeConfig{Port: "8080"},
			run:  func(*Runtime) error { return nil },
			want: "logger",
		},
		{
			name: "port",
			ctx:  context.Background(),
			deps: ServeConfig{Logger: slog.Default(), FrontendURL: "http://localhost:3000"},
			run:  func(*Runtime) error { return nil },
			want: "port",
		},
		{
			name: "frontend URL",
			ctx:  context.Background(),
			deps: ServeConfig{Port: "8080", Logger: slog.Default()},
			run:  func(*Runtime) error { return nil },
			want: "frontend URL",
		},
		{
			name: "context",
			deps: ServeConfig{Port: "8080", Logger: slog.Default()},
			run:  func(*Runtime) error { return nil },
			want: "context",
		},
		{
			name: "callback",
			ctx:  context.Background(),
			deps: ServeConfig{Port: "8080", Logger: slog.Default()},
			want: "callback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := WithRuntime(tt.ctx, tt.deps, tt.run)

			require.ErrorContains(t, err, tt.want)
		})
	}

	t.Run("cancelled before build", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		called := false

		err := WithRuntime(ctx, ServeConfig{Port: "8080", Logger: slog.Default()}, func(*Runtime) error {
			called = true
			return nil
		})

		require.ErrorIs(t, err, context.Canceled)
		require.False(t, called)
	})
}

func TestStaffMessageCleanupSkipsMissingRuntime(t *testing.T) {
	t.Parallel()

	require.Nil(t, staffMessageCleanup(nil))
}

func TestRuntimeServeReturnsListenFailure(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, listener.Close()) })

	runtime := &Runtime{
		server: &http.Server{Addr: listener.Addr().String()},
		logger: slog.Default(),
	}

	err = runtime.Serve(context.Background())

	require.ErrorContains(t, err, "listen on")
}

func TestRuntimeServeDoesNotStartBackgroundAfterCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := make(chan struct{})
	runtime := &Runtime{
		server: &http.Server{Addr: "127.0.0.1:0"},
		worker: recordingWorker{started: started},
		logger: slog.Default(),
	}

	require.NoError(t, runtime.Serve(ctx))
	select {
	case <-started:
		t.Fatal("scheduler started after Serve context cancellation")
	default:
	}
}

func TestRuntimeServeStartsWorkerAfterHTTPReadiness(t *testing.T) {
	t.Parallel()

	reserved, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = reserved.Close() })
	addr := reserved.Addr().String()

	workerStarted := make(chan error, 1)
	runtime := &Runtime{
		server: &http.Server{
			Addr:    addr,
			Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
		},
		worker: readinessWorker{url: "http://" + addr, started: workerStarted},
		logger: slog.Default(),
		listen: func(_, _ string) (net.Listener, error) {
			return reserved, nil
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	serveDone := make(chan error, 1)
	go func() { serveDone <- runtime.Serve(ctx) }()

	select {
	case err := <-workerStarted:
		require.NoError(t, err)
	case err := <-serveDone:
		require.NoError(t, err)
		t.Fatal("server stopped before worker readiness check")
	case <-time.After(time.Second):
		t.Fatal("worker did not start after HTTP readiness")
	}
	cancel()
	require.NoError(t, <-serveDone)
}

func TestRuntimeShutdownReturnsDeadlineForStuckWorker(t *testing.T) {
	t.Parallel()

	releaseWorker := make(chan struct{})
	runtime := &Runtime{
		server: &http.Server{},
		worker: blockingWorker{release: releaseWorker},
		logger: slog.Default(),
	}
	shutdownDone := make(chan error, 1)
	go func() {
		shutdownDone <- runtime.shutdownWithTimeout(errors.New("test shutdown"), 50*time.Millisecond)
	}()

	err := <-shutdownDone
	close(releaseWorker)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.True(t, runtime.resourcesUnsafe)
}

func TestRuntimeShutdownLetsServerCloseItsListener(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	runtime := &Runtime{
		server: &http.Server{Handler: http.NotFoundHandler()},
		logger: slog.Default(),
	}
	serveDone := make(chan error, 1)
	go func() { serveDone <- runtime.server.Serve(listener) }()

	require.Eventually(t, func() bool {
		connection, err := net.Dial("tcp", listener.Addr().String())
		if err != nil {
			return false
		}
		return connection.Close() == nil
	}, time.Second, 10*time.Millisecond)

	require.NoError(t, runtime.shutdown(errors.New("test shutdown")))
	require.ErrorIs(t, <-serveDone, http.ErrServerClosed)
}

func TestRuntimeServeReturnsNilAfterCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	runtime := &Runtime{
		server: &http.Server{Addr: "127.0.0.1:0"},
		logger: slog.Default(),
	}
	serveDone := make(chan error, 1)
	go func() { serveDone <- runtime.Serve(ctx) }()

	cancel()
	require.NoError(t, <-serveDone)
}
