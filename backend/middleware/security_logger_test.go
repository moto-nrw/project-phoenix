package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// One limiter rejection yields exactly one RATE_LIMIT_EXCEEDED event, and a
// 429 a handler writes itself (an MFA send limit, a password-reset window)
// yields none: the limiter is the only source of the event.
func TestRateLimitRejectionLogsOneSecurityEvent(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	sl := &SecurityLogger{logger: slog.New(slog.NewTextHandler(&buf, nil))}
	limiter := NewRateLimiter(1, 1)
	limiter.SetLogger(sl)

	r := chi.NewRouter()
	r.Use(limiter.Middleware())
	r.Get("/limited", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	r.Post("/mfa/send", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})

	serve := func(method, path, remoteAddr string) int {
		req := httptest.NewRequest(method, path, nil)
		req.RemoteAddr = remoteAddr
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		return rr.Code
	}

	if got := serve(http.MethodPost, "/mfa/send", "203.0.113.20:1000"); got != http.StatusTooManyRequests {
		t.Fatalf("handler 429: status = %d, want 429", got)
	}
	if n := strings.Count(buf.String(), EventRateLimitExceed); n != 0 {
		t.Fatalf("handler-level 429 logged %d security events, want 0:\n%s", n, buf.String())
	}

	if got := serve(http.MethodGet, "/limited", "203.0.113.10:1000"); got != http.StatusNoContent {
		t.Fatalf("first request: status = %d, want 204", got)
	}
	if got := serve(http.MethodGet, "/limited", "203.0.113.10:1000"); got != http.StatusTooManyRequests {
		t.Fatalf("second request: status = %d, want 429", got)
	}
	if n := strings.Count(buf.String(), EventRateLimitExceed); n != 1 {
		t.Fatalf("one rejection logged %d security events, want 1:\n%s", n, buf.String())
	}
}
