package api

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

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
