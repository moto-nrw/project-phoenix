package middleware

import (
	"context"
	"log/slog"
	"strings"
)

// feedCalendarPathMarker is the public subscription feed prefix. The path
// segment after it is the parent's iCalendar token — the sole credential for the
// unauthenticated feed, so it must never be written to logs where it could be
// replayed.
const feedCalendarPathMarker = "/public/calendar/"

// attrRedactor wraps a slog.Handler and rewrites the message and every string
// attribute (including nested groups) through transform. transform receives the
// attribute key ("" for the record message) so implementations can redact
// selectively.
type attrRedactor struct {
	inner     slog.Handler
	transform func(key, value string) string
}

// NewFeedTokenRedactor wraps inner so calendar-feed tokens are redacted from
// every emitted log record (message, string attributes, and nested groups). It
// is applied to both the HTTP request logger and the security logger, whose
// "path" attribute would otherwise capture the token verbatim (the security
// logger records it on rate-limited requests).
func NewFeedTokenRedactor(inner slog.Handler) slog.Handler {
	return &attrRedactor{inner: inner, transform: func(_, value string) string {
		return RedactFeedToken(value)
	}}
}

// NewQueryValueRedactor wraps inner so query-string values never reach the
// access log (issue #2105): staff-UI search endpoints carry student names and
// e-mail addresses as query parameters. The "query" attribute keeps only the
// parameter names; the "referer" attribute keeps its path but loses its query
// values the same way. Method, path, status, and duration stay intact.
func NewQueryValueRedactor(inner slog.Handler) slog.Handler {
	return &attrRedactor{inner: inner, transform: func(key, value string) string {
		switch key {
		case "query":
			return RedactQueryValues(value)
		case "referer":
			if path, query, found := strings.Cut(value, "?"); found {
				return path + "?" + RedactQueryValues(query)
			}
			return value
		default:
			return value
		}
	}}
}

func (h *attrRedactor) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *attrRedactor) Handle(ctx context.Context, rec slog.Record) error {
	out := slog.NewRecord(rec.Time, rec.Level, h.transform("", rec.Message), rec.PC)
	rec.Attrs(func(a slog.Attr) bool {
		out.AddAttrs(h.redactAttr(a))
		return true
	})
	return h.inner.Handle(ctx, out)
}

func (h *attrRedactor) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		redacted[i] = h.redactAttr(a)
	}
	return &attrRedactor{inner: h.inner.WithAttrs(redacted), transform: h.transform}
}

func (h *attrRedactor) WithGroup(name string) slog.Handler {
	return &attrRedactor{inner: h.inner.WithGroup(name), transform: h.transform}
}

func (h *attrRedactor) redactAttr(a slog.Attr) slog.Attr {
	switch a.Value.Kind() {
	case slog.KindString:
		return slog.String(a.Key, h.transform(a.Key, a.Value.String()))
	case slog.KindGroup:
		group := a.Value.Group()
		redacted := make([]slog.Attr, len(group))
		for i, ga := range group {
			redacted[i] = h.redactAttr(ga)
		}
		return slog.Attr{Key: a.Key, Value: slog.GroupValue(redacted...)}
	default:
		return a
	}
}

// RedactFeedToken replaces the token segment of any "/public/calendar/<token>"
// occurrence in s with "[REDACTED]", preserving any trailing path or query.
func RedactFeedToken(s string) string {
	idx := strings.Index(s, feedCalendarPathMarker)
	if idx < 0 {
		return s
	}
	start := idx + len(feedCalendarPathMarker)
	end := start
	for end < len(s) {
		if c := s[end]; c == '/' || c == '?' || c == ' ' || c == '"' {
			break
		}
		end++
	}
	if end == start {
		return s
	}
	return s[:start] + "[REDACTED]" + s[end:]
}

// RedactQueryValues strips the values from a raw query string, keeping only
// the parameter names: "search=Mustermann&page=2" becomes "search&page". Names
// alone are enough for diagnosis; values may contain personal data.
func RedactQueryValues(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}
	parts := strings.Split(rawQuery, "&")
	for i, p := range parts {
		if name, _, found := strings.Cut(p, "="); found {
			parts[i] = name
		}
	}
	return strings.Join(parts, "&")
}
