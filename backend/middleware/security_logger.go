package middleware

import (
	"cmp"
	"log/slog"
	"net/http"
	"time"
)

// SecurityEvent types
const EventRateLimitExceed = "RATE_LIMIT_EXCEEDED"

// SecurityLogger provides structured security event logging
type SecurityLogger struct {
	logger *slog.Logger
}

// getLogger returns a nil-safe logger, falling back to slog.Default() if logger is nil
func (sl *SecurityLogger) getLogger() *slog.Logger {
	return cmp.Or(sl.logger, slog.Default())
}

// NewSecurityLogger creates a new security logger
func NewSecurityLogger() *SecurityLogger {
	// Redact calendar-feed tokens: security events log r.URL.Path, which for a
	// rate-limited /public/calendar/{token} request would otherwise capture the
	// token (the feed's sole credential).
	redacted := slog.New(NewFeedTokenRedactor(slog.Default().Handler()))
	return &SecurityLogger{logger: redacted.With("component", "security")}
}

// LogEvent logs a security event with context
func (sl *SecurityLogger) LogEvent(eventType string, r *http.Request, details map[string]interface{}) {
	ip := GetClientIP(r)
	userAgent := r.Header.Get("User-Agent")

	attrs := []any{
		"event", eventType,
		"ip", ip,
		"method", r.Method,
		"path", r.URL.Path,
		"user_agent", userAgent,
	}
	for k, v := range details {
		attrs = append(attrs, k, v)
	}

	sl.getLogger().Info("security event", attrs...)
}

// LogRateLimitExceeded logs rate limit violations
func (sl *SecurityLogger) LogRateLimitExceeded(r *http.Request) {
	sl.LogEvent(EventRateLimitExceed, r, map[string]interface{}{
		"timestamp": time.Now().Unix(),
	})
}

// SecurityLoggingMiddleware logs security-relevant requests
func SecurityLoggingMiddleware(sl *SecurityLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Wrap response writer to capture status code
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(wrapped, r)

			// Log based on response
			if wrapped.statusCode == http.StatusTooManyRequests {
				sl.LogRateLimitExceeded(r)
			}
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Flush implements http.Flusher by delegating to the underlying ResponseWriter.
// This is required for SSE (Server-Sent Events) to work when this middleware is active.
func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
