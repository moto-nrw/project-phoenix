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

// feedTokenRedactor wraps a slog.Handler and rewrites the calendar-feed
// subscription token in any log record (message, string attributes, and nested
// groups) to "[REDACTED]". It is applied to both the HTTP request logger and the
// security logger, whose "path" attribute would otherwise capture the token
// verbatim (the security logger records it on rate-limited requests).
type feedTokenRedactor struct {
	inner slog.Handler
}

// NewFeedTokenRedactor wraps inner so calendar-feed tokens are redacted from
// every emitted log record.
func NewFeedTokenRedactor(inner slog.Handler) slog.Handler {
	return &feedTokenRedactor{inner: inner}
}

func (h *feedTokenRedactor) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *feedTokenRedactor) Handle(ctx context.Context, rec slog.Record) error {
	out := slog.NewRecord(rec.Time, rec.Level, RedactFeedToken(rec.Message), rec.PC)
	rec.Attrs(func(a slog.Attr) bool {
		out.AddAttrs(redactFeedTokenAttr(a))
		return true
	})
	return h.inner.Handle(ctx, out)
}

func (h *feedTokenRedactor) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		redacted[i] = redactFeedTokenAttr(a)
	}
	return &feedTokenRedactor{inner: h.inner.WithAttrs(redacted)}
}

func (h *feedTokenRedactor) WithGroup(name string) slog.Handler {
	return &feedTokenRedactor{inner: h.inner.WithGroup(name)}
}

func redactFeedTokenAttr(a slog.Attr) slog.Attr {
	switch a.Value.Kind() {
	case slog.KindString:
		return slog.String(a.Key, RedactFeedToken(a.Value.String()))
	case slog.KindGroup:
		group := a.Value.Group()
		redacted := make([]slog.Attr, len(group))
		for i, ga := range group {
			redacted[i] = redactFeedTokenAttr(ga)
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
