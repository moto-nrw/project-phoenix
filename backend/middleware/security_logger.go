package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// SecurityEvent types
const (
	EventAuthFailure      = "AUTH_FAILURE"
	EventRateLimitExceed  = "RATE_LIMIT_EXCEEDED"
	EventSuspiciousAccess = "SUSPICIOUS_ACCESS"
	EventAccountLocked    = "ACCOUNT_LOCKED"
	EventInvalidToken     = "INVALID_TOKEN"
)

// SecurityLogger provides structured security event logging
type SecurityLogger struct {
	logger *slog.Logger
}

// getLogger returns a nil-safe logger, falling back to slog.Default() if logger is nil
func (sl *SecurityLogger) getLogger() *slog.Logger {
	if sl.logger != nil {
		return sl.logger
	}
	return slog.Default()
}

// NewSecurityLogger creates a new security logger
func NewSecurityLogger() *SecurityLogger {
	return &SecurityLogger{logger: slog.Default().With("component", "security")}
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
