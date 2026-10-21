package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// SecurityLoggingMiddleware Tests
// =============================================================================

func TestSecurityLoggingMiddleware_VariousStatusCodes(t *testing.T) {
	t.Parallel()

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
