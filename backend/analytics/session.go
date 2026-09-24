package analytics

import (
	"context"
	"regexp"
)

// SessionIDHeader carries the browser's PostHog session ID. posthog-js adds
// it to requests to the own host (tracing_headers); the Next.js route
// handlers pass it on to the backend.
const SessionIDHeader = "X-POSTHOG-SESSION-ID"

// sessionIDPattern accepts the UUIDs posthog-js generates for sessions and
// nothing else, so the header cannot smuggle free text into an event.
var sessionIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type sessionIDKey struct{}

// WithSessionID stores the browser session of a request. A value that is not
// a session UUID is ignored.
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	if !sessionIDPattern.MatchString(sessionID) {
		return ctx
	}
	return context.WithValue(ctx, sessionIDKey{}, sessionID)
}

// SessionIDFromContext returns the browser session of the request, or "".
func SessionIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	sessionID, _ := ctx.Value(sessionIDKey{}).(string)
	return sessionID
}
