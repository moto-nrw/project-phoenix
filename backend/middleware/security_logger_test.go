package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// SecurityLogger Tests
// =============================================================================

func TestNewSecurityLogger(t *testing.T) {
	sl := NewSecurityLogger()
	assert.NotNil(t, sl)
	assert.NotNil(t, sl.logger)
}

func TestSecurityLogger_LogEvent(t *testing.T) {
	sl := NewSecurityLogger()

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("User-Agent", "TestAgent/1.0")
	req.Header.Set("X-Forwarded-For", "192.168.1.1")

	// Should not panic
	assert.NotPanics(t, func() {
		sl.LogEvent(EventAuthFailure, req, map[string]interface{}{
			"user_id": 123,
			"reason":  "invalid password",
		})
	})
}

func TestSecurityLogger_LogEvent_EmptyDetails(t *testing.T) {
	sl := NewSecurityLogger()

	req := httptest.NewRequest(http.MethodPost, "/login", nil)

	assert.NotPanics(t, func() {
		sl.LogEvent(EventAuthFailure, req, nil)
	})
}

func TestSecurityLogger_LogEvent_AllEventTypes(t *testing.T) {
	sl := NewSecurityLogger()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	events := []string{
		EventAuthFailure,
		EventRateLimitExceed,
		EventSuspiciousAccess,
		EventAccountLocked,
		EventInvalidToken,
	}

	for _, eventType := range events {
		t.Run(eventType, func(t *testing.T) {
			assert.NotPanics(t, func() {
				sl.LogEvent(eventType, req, nil)
			})
		})
	}
}

func TestSecurityLogger_LogRateLimitExceeded(t *testing.T) {
	sl := NewSecurityLogger()

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"

	assert.NotPanics(t, func() {
		sl.LogRateLimitExceeded(req)
	})
}

// =============================================================================
// SecurityLoggingMiddleware Tests
// =============================================================================

func TestSecurityLoggingMiddleware_NormalRequest(t *testing.T) {
	sl := NewSecurityLogger()

	r := chi.NewRouter()
	r.Use(SecurityLoggingMiddleware(sl))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestSecurityLoggingMiddleware_RateLimitExceeded(t *testing.T) {
	sl := NewSecurityLogger()

	r := chi.NewRouter()
	r.Use(SecurityLoggingMiddleware(sl))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func TestSecurityLoggingMiddleware_VariousStatusCodes(t *testing.T) {
	sl := NewSecurityLogger()

	statusCodes := []int{
		http.StatusOK,
		http.StatusCreated,
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
	}

	for _, code := range statusCodes {
		t.Run(http.StatusText(code), func(t *testing.T) {
			r := chi.NewRouter()
			r.Use(SecurityLoggingMiddleware(sl))
			r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			assert.Equal(t, code, rr.Code)
		})
	}
}

// =============================================================================
// responseWriter Tests
// =============================================================================

func TestResponseWriter_WriteHeader(t *testing.T) {
	rr := httptest.NewRecorder()
	wrapped := &responseWriter{ResponseWriter: rr, statusCode: http.StatusOK}

	wrapped.WriteHeader(http.StatusNotFound)

	assert.Equal(t, http.StatusNotFound, wrapped.statusCode)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestResponseWriter_DefaultStatusCode(t *testing.T) {
	rr := httptest.NewRecorder()
	wrapped := &responseWriter{ResponseWriter: rr, statusCode: http.StatusOK}

	// Before WriteHeader is called, status should be default
	assert.Equal(t, http.StatusOK, wrapped.statusCode)
}

func TestResponseWriter_Write(t *testing.T) {
	rr := httptest.NewRecorder()
	wrapped := &responseWriter{ResponseWriter: rr, statusCode: http.StatusOK}

	n, err := wrapped.Write([]byte("test body"))

	assert.NoError(t, err)
	assert.Equal(t, 9, n)
	assert.Equal(t, "test body", rr.Body.String())
}

// =============================================================================
// Event Constants Tests
// =============================================================================

func TestEventConstants(t *testing.T) {
	assert.Equal(t, "AUTH_FAILURE", EventAuthFailure)
	assert.Equal(t, "RATE_LIMIT_EXCEEDED", EventRateLimitExceed)
	assert.Equal(t, "SUSPICIOUS_ACCESS", EventSuspiciousAccess)
	assert.Equal(t, "ACCOUNT_LOCKED", EventAccountLocked)
	assert.Equal(t, "INVALID_TOKEN", EventInvalidToken)
}
