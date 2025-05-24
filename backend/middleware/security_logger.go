package middleware

import (
	"fmt"
	"log"
	"net/http"
	"os"
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
	logger *log.Logger
}

// NewSecurityLogger creates a new security logger
func NewSecurityLogger() *SecurityLogger {
	// In production, this could write to a separate security log file
	logger := log.New(os.Stdout, "[SECURITY] ", log.LstdFlags|log.Lshortfile)
	return &SecurityLogger{logger: logger}
}

// LogEvent logs a security event with context
func (sl *SecurityLogger) LogEvent(eventType string, r *http.Request, details map[string]interface{}) {
	ip := GetClientIP(r)
	userAgent := r.Header.Get("User-Agent")
	
	logEntry := fmt.Sprintf("event=%s ip=%s method=%s path=%s ua=%q",
		eventType, ip, r.Method, r.URL.Path, userAgent)
	
	// Add additional details
	for k, v := range details {
		logEntry += fmt.Sprintf(" %s=%v", k, v)
	}
	
	sl.logger.Println(logEntry)
}

// LogAuthFailure logs authentication failures
func (sl *SecurityLogger) LogAuthFailure(r *http.Request, email string, reason string) {
	sl.LogEvent(EventAuthFailure, r, map[string]interface{}{
		"email":     email,
		"reason":    reason,
		"timestamp": time.Now().Unix(),
	})
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
			// TODO: Add suspicious pattern detection for login attempts
			// e.g., SQL injection attempts, unusual payloads, etc.
			
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