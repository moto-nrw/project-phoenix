package middleware

import (
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// SecurityEvent types
const (
	EventAuthFailure      = "AUTH_FAILURE"
	EventRateLimitExceed  = "RATE_LIMIT_EXCEEDED"
	EventSuspiciousAccess = "SUSPICIOUS_ACCESS"
	EventAccountLocked    = "ACCOUNT_LOCKED"
	EventInvalidToken     = "INVALID_TOKEN"
)

// SecurityLogger provides structured security event logging using logrus.
// This follows 12-Factor App Factor 11 (Logs): treat logs as event streams.
type SecurityLogger struct {
	logger *logrus.Entry
}

// NewSecurityLogger creates a new security logger.
// If logger is nil, it uses logrus.StandardLogger().
func NewSecurityLogger(logger *logrus.Logger) *SecurityLogger {
	if logger == nil {
		logger = logrus.StandardLogger()
	}
	// Create a dedicated entry with security prefix for easy filtering
	entry := logger.WithField("component", "security")
	return &SecurityLogger{logger: entry}
}

// LogEvent logs a security event with context
func (sl *SecurityLogger) LogEvent(eventType string, r *http.Request, details map[string]any) {
	ip := GetClientIP(r)
	userAgent := r.Header.Get("User-Agent")

	fields := logrus.Fields{
		"event":      eventType,
		"ip":         ip,
		"method":     r.Method,
		"path":       r.URL.Path,
		"user_agent": userAgent,
	}

	// Add additional details
	for k, v := range details {
		fields[k] = v
	}

	sl.logger.WithFields(fields).Warn("security event")
}

// LogRateLimitExceeded logs rate limit violations
func (sl *SecurityLogger) LogRateLimitExceeded(r *http.Request) {
	sl.LogEvent(EventRateLimitExceed, r, map[string]any{
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
