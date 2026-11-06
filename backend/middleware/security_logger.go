package middleware

import (
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
	if sl.logger == nil {
		return slog.Default()
	}
	return sl.logger
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

// LogRateLimitExceeded logs a rate limit rejection. The RateLimiter is its
// only caller, so each rejection yields one event; a 429 a handler writes
// itself (e.g. an MFA send limit) is no limiter rejection and is not logged.
func (sl *SecurityLogger) LogRateLimitExceeded(r *http.Request) {
	sl.LogEvent(EventRateLimitExceed, r, map[string]interface{}{
		"timestamp": time.Now().Unix(),
	})
}
