package api

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestRedactFeedToken(t *testing.T) {
	cases := map[string]string{
		"/public/calendar/secret-token":     "/public/calendar/[REDACTED]",
		"/public/calendar/tok123?refresh=1": "/public/calendar/[REDACTED]?refresh=1",
		"GET /public/calendar/abc 200 12ms": "GET /public/calendar/[REDACTED] 200 12ms",
		"/public/calendar/":                 "/public/calendar/", // no token segment
		"/api/calendar/appointments/5/ics":  "/api/calendar/appointments/5/ics",
		"nothing sensitive here":            "nothing sensitive here",
	}
	for in, want := range cases {
		if got := redactFeedToken(in); got != want {
			t.Errorf("redactFeedToken(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFeedTokenRedactorHandler(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewTextHandler(&buf, &slog.HandlerOptions{})
	logger := slog.New(newFeedTokenRedactor(base))

	// A grouped "request" attribute (as the request logger emits) must be redacted.
	logger.Info("request",
		slog.Group("request",
			slog.String("method", "GET"),
			slog.String("path", "/public/calendar/leaky-token"),
		),
	)
	// A top-level path attribute (as the security logger emits) must be redacted.
	logger.Info("security event", slog.String("path", "/public/calendar/another-token"))

	out := buf.String()
	if strings.Contains(out, "leaky-token") || strings.Contains(out, "another-token") {
		t.Errorf("feed token leaked into logs:\n%s", out)
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Errorf("expected [REDACTED] in logs:\n%s", out)
	}
	// A non-feed path is untouched.
	buf.Reset()
	logger.Info("other", slog.String("path", "/api/staff"))
	if !strings.Contains(buf.String(), "/api/staff") {
		t.Errorf("non-feed path should be preserved:\n%s", buf.String())
	}
}

// ensure the handler satisfies the interface at compile time
var _ slog.Handler = (*feedTokenRedactor)(nil)

func TestFeedTokenRedactorEnabled(t *testing.T) {
	h := newFeedTokenRedactor(slog.NewTextHandler(&bytes.Buffer{}, nil))
	if !h.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("expected Info to be enabled")
	}
}
