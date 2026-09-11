package middleware

import (
	"bytes"
	"context"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRedactFeedToken(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"/public/calendar/secret-token":                     "/public/calendar/[REDACTED]",
		"/public/request-feed/rss-secret":                   "/public/request-feed/[REDACTED]",
		"/public/calendar/tok123?refresh=1":                 "/public/calendar/[REDACTED]?refresh=1",
		"GET /public/calendar/abc 200 12ms":                 "GET /public/calendar/[REDACTED] 200 12ms",
		"/public/calendar/one /public/request-feed/two":     "/public/calendar/[REDACTED] /public/request-feed/[REDACTED]",
		"/public/request-feed/one /public/request-feed/two": "/public/request-feed/[REDACTED] /public/request-feed/[REDACTED]",
		"/public/calendar/":                                 "/public/calendar/", // no token segment
		"/api/calendar/appointments/5/ics":                  "/api/calendar/appointments/5/ics",
		"nothing sensitive here":                            "nothing sensitive here",
	}
	for in, want := range cases {
		if got := RedactFeedToken(in); got != want {
			t.Errorf("RedactFeedToken(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFeedTokenRedactorHandler(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	base := slog.NewTextHandler(&buf, &slog.HandlerOptions{})
	logger := slog.New(NewFeedTokenRedactor(base))

	// A grouped "request" attribute (as the request logger emits) must be redacted.
	logger.Info("request",
		slog.Group("request",
			slog.String("method", "GET"),
			slog.String("path", "/public/calendar/leaky-token"),
			slog.Any("params", map[string]string{"token": "leaky-param-token"}),
		),
	)
	// A top-level path attribute (as the security logger emits) must be redacted.
	logger.Info("security event", slog.String("path", "/public/calendar/another-token"))

	out := buf.String()
	if strings.Contains(out, "leaky-token") || strings.Contains(out, "leaky-param-token") || strings.Contains(out, "another-token") {
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

// The security logger derived from NewSecurityLogger must redact feed tokens.
func TestSecurityLoggerRedactsFeedToken(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	sl := &SecurityLogger{logger: slog.New(NewFeedTokenRedactor(slog.NewTextHandler(&buf, nil)))}

	req := httptest.NewRequest("GET", "/public/calendar/rate-limited-token", nil)
	sl.LogRateLimitExceeded(req)

	out := buf.String()
	if strings.Contains(out, "rate-limited-token") {
		t.Errorf("security logger leaked feed token:\n%s", out)
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Errorf("expected redacted path in security log:\n%s", out)
	}
}

func TestRedactQueryValues(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"search=Mustermann":               "search",
		"search=Mustermann&page=2":        "search&page",
		"first_name=Max&last_name=Muster": "first_name&last_name",
		"email=max%40example.com":         "email",
		"flag":                            "flag", // valueless parameter stays
		"":                                "",
		"a=1&b&c=x=y":                     "a&b&c", // value containing '=' is dropped entirely
	}
	for in, want := range cases {
		if got := RedactQueryValues(in); got != want {
			t.Errorf("RedactQueryValues(%q) = %q, want %q", in, got, want)
		}
	}
}

// Issue #2105: the access-log "query" attribute (nested in slog-chi's
// "request" group) and the "referer" attribute must lose their query values
// while other attributes stay intact.
func TestQueryValueRedactorHandler(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(NewQueryValueRedactor(slog.NewTextHandler(&buf, nil)))

	logger.Info("request",
		slog.Group("request",
			slog.String("method", "GET"),
			slog.String("path", "/api/students"),
			slog.String("query", "search=Mustermann&page=2"),
			slog.String("referer", "https://schule.example.com/students/search?search=Mustermann"),
		),
	)

	out := buf.String()
	if strings.Contains(out, "Mustermann") {
		t.Errorf("query value leaked into logs:\n%s", out)
	}
	for _, want := range []string{"query=search&page", "/api/students", "method=GET", "referer=https://schule.example.com/students/search?search"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in logs:\n%s", want, out)
		}
	}
	// Non-query attributes containing '=' or '&' are untouched.
	buf.Reset()
	logger.Info("other", slog.String("detail", "a=1&b=2"))
	if !strings.Contains(buf.String(), "a=1&b=2") {
		t.Errorf("non-query attribute should be preserved:\n%s", buf.String())
	}
}

var _ slog.Handler = (*attrRedactor)(nil)

func TestFeedTokenRedactorEnabled(t *testing.T) {
	t.Parallel()

	h := NewFeedTokenRedactor(slog.NewTextHandler(&bytes.Buffer{}, nil))
	if !h.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("expected Info to be enabled")
	}
}
