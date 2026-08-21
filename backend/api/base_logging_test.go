package api

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
)

// captureHandler is a minimal slog.Handler that records every emitted record
// so tests can assert which requests the logging middleware logged.
type captureHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r)
	return nil
}

func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(string) slog.Handler      { return h }

func (h *captureHandler) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.records)
}

// TestLoggingMiddlewareIgnoresScannerMethods guards the issue #850 fix: probe
// methods our API never serves (CONNECT, TRACE, PRI) must not produce log
// noise, while legitimate-method requests that 4xx must still be logged.
func TestLoggingMiddlewareIgnoresScannerMethods(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		path       string
		wantLogged bool
	}{
		{name: "CONNECT probe is dropped", method: http.MethodConnect, path: "/", wantLogged: false},
		{name: "TRACE probe is dropped", method: http.MethodTrace, path: "/", wantLogged: false},
		{name: "PRI HTTP/2 preface is dropped", method: "PRI", path: "*", wantLogged: false},
		{name: "GET 404 on bogus path is logged", method: http.MethodGet, path: "/wp-login.php", wantLogged: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capture := &captureHandler{}
			logger := slog.New(capture)

			router := chi.NewRouter()
			setupBasicMiddleware(router, logger, nil)
			router.Get("/", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(tt.method, "http://example.com"+tt.path, nil)
			router.ServeHTTP(httptest.NewRecorder(), req)

			got := capture.count() > 0
			if got != tt.wantLogged {
				t.Fatalf("method %s path %s: logged=%v, want %v (records=%d)",
					tt.method, tt.path, got, tt.wantLogged, capture.count())
			}
		})
	}
}
