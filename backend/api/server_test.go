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

type blockingScheduler struct {
	release <-chan struct{}
}

func (scheduler blockingScheduler) Start() {}

func (scheduler blockingScheduler) Stop() {
	<-scheduler.release
}

func TestWithRuntimeRejectsMissingDependencies(t *testing.T) {
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
			deps: ServeConfig{Logger: slog.Default()},
			run:  func(*Runtime) error { return nil },
			want: "port",
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

func TestRuntimeShutdownWaitsForSchedulerBeforeReturning(t *testing.T) {
	t.Parallel()

	releaseScheduler := make(chan struct{})
	runtime := &Runtime{
		server:    &http.Server{},
		scheduler: blockingScheduler{release: releaseScheduler},
		logger:    slog.Default(),
	}
	shutdownDone := make(chan error, 1)
	go func() {
		shutdownDone <- runtime.shutdownWithTimeout(errors.New("test shutdown"), 50*time.Millisecond)
	}()

	select {
	case err := <-shutdownDone:
		t.Fatalf("shutdown returned before scheduler stopped: %v", err)
	case <-time.After(75 * time.Millisecond):
	}

	close(releaseScheduler)
	require.NoError(t, <-shutdownDone)
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
