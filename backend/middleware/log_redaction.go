package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
)

// feedCalendarPathMarker is the public subscription feed prefix. The path
// The segment after either marker is the sole credential for an unauthenticated
// feed, so it must never be written to logs where it could be replayed.
var feedPathMarkers = []string{"/public/calendar/", "/public/request-feed/"}

// invitationPathMarkers precede a school's ID and then the caterer's mailed
// invitation token (ADR 0077): the token alone accepts the invitation.
var invitationPathMarkers = []string{"/caterer/invitations/", "/einladung/"}

// Signing links carry a replayable contract credential. Match escaped path
// prefixes too: referrers and error texts can retain their URL encoding.
var signingTokenPattern = regexp.MustCompile(`(?i)(` + encodedPathMarker("/public/care-contract-signing/") + `|` +
	encodedPathMarker("/vertrag-unterschreiben/") + `)[^/?#\s"']+`)

// tokenPathPattern matches the segment after a route prefix whose token is the
// only credential of an unauthenticated request: enrollment status links,
// staff, guardian and Träger invitations, holiday registration and quote
// links, and payroll download links. Some prefixes also serve signed-in
// routes with a numeric ID or a fixed word in that position; isRouteID keeps
// those readable.
var tokenPathPattern = regexp.MustCompile(`(/api/enrollment/requests/|/auth/invitations/|/auth/guardian-invitations/|` +
	`/traeger/invitations/|/traeger/holiday-registrations/|/traeger/holiday-quotes/|/parent/holiday-registrations/|` +
	`/public/payroll-download/)([^/?#\s"']+)`)

// fixedTokenPathSegments are the route words that share a position with a token.
// The operator's invitation routes take their token in the body.
var fixedTokenPathSegments = map[string]bool{"former-employment": true, "accept": true, "validate": true}

// isRouteID reports whether a path segment is a numeric ID or a fixed route
// word rather than a token. An unknown word is redacted: a lost word costs a
// log line some detail, a logged token can be replayed.
func isRouteID(segment string) bool {
	if fixedTokenPathSegments[segment] {
		return true
	}
	for _, char := range segment {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func redactTokenPaths(s string) string {
	return tokenPathPattern.ReplaceAllStringFunc(s, func(match string) string {
		parts := tokenPathPattern.FindStringSubmatch(match)
		if isRouteID(parts[2]) {
			return match
		}
		return parts[1] + "[REDACTED]"
	})
}

func encodedPathMarker(path string) string {
	var pattern strings.Builder
	for _, char := range path {
		fmt.Fprintf(&pattern, "(?:%s|%%%02x)", regexp.QuoteMeta(string(char)), char)
	}
	return pattern.String()
}

// attrRedactor wraps a slog.Handler and rewrites the message and every string
// attribute (including nested groups) through transform. transform receives the
// attribute key ("" for the record message) so implementations can redact
// selectively.
type attrRedactor struct {
	inner     slog.Handler
	transform func(key, value string) string
}

// NewFeedTokenRedactor wraps inner so public URL credentials are redacted from
// every emitted log record (message, string attributes, and nested groups). It
// is applied to both the HTTP request logger and the security logger, whose
// "path" attribute would otherwise capture the token verbatim (the security
// logger records it on rate-limited requests).
func NewFeedTokenRedactor(inner slog.Handler) slog.Handler {
	return &attrRedactor{inner: inner, transform: func(key, value string) string {
		if key == "token" || key == "link" {
			return "[REDACTED]"
		}
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
	case slog.KindAny:
		if values, ok := a.Value.Any().([]string); ok {
			redacted := make([]string, len(values))
			for i, value := range values {
				redacted[i] = h.transform(a.Key, value)
			}
			return slog.Any(a.Key, redacted)
		}
		if values, ok := a.Value.Any().(map[string]string); ok {
			redacted := make(map[string]string, len(values))
			for key, value := range values {
				redacted[key] = h.transform(key, value)
			}
			return slog.Any(a.Key, redacted)
		}
		return a
	default:
		return a
	}
}

// RedactFeedToken replaces the credential segment of a public feed, invitation,
// status-link or contract-signing URL with "[REDACTED]", preserving any
// trailing path or query. TestRouteCredentialsStayOutOfLogs holds it to the
// route table: a new route with a token parameter fails there until it is
// covered here.
func RedactFeedToken(s string) string {
	for _, marker := range feedPathMarkers {
		s = redactAfter(s, marker, 0)
	}
	for _, marker := range invitationPathMarkers {
		s = redactAfter(s, marker, 1)
	}
	s = redactTokenPaths(s)
	return signingTokenPattern.ReplaceAllString(s, "${1}[REDACTED]")
}

// redactAfter replaces the path segment that follows marker and skip more
// segments, wherever marker occurs.
func redactAfter(s, marker string, skip int) string {
	searchFrom := 0
	for {
		relative := strings.Index(s[searchFrom:], marker)
		if relative < 0 {
			return s
		}
		start := searchFrom + relative + len(marker)
		for range skip {
			start = segmentEnd(s, start)
			if start >= len(s) || s[start] != '/' {
				break
			}
			start++
		}
		end := segmentEnd(s, start)
		if end == start {
			searchFrom = start
			continue
		}
		s = s[:start] + "[REDACTED]" + s[end:]
		searchFrom = start + len("[REDACTED]")
	}
}

// segmentEnd is where the path segment starting at start ends.
func segmentEnd(s string, start int) int {
	end := start
	for end < len(s) {
		if c := s[end]; c == '/' || c == '?' || c == ' ' || c == '"' {
			break
		}
		end++
	}
	return end
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
