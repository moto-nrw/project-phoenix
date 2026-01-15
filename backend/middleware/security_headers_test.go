package middleware

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// SecurityHeaders Middleware Tests
// =============================================================================

func TestSecurityHeaders_SetsXFrameOptions(t *testing.T) {
	r := chi.NewRouter()
	r.Use(SecurityHeaders)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, "DENY", rr.Header().Get("X-Frame-Options"))
}

func TestSecurityHeaders_SetsXContentTypeOptions(t *testing.T) {
	r := chi.NewRouter()
	r.Use(SecurityHeaders)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, "nosniff", rr.Header().Get("X-Content-Type-Options"))
}

func TestSecurityHeaders_SetsXXSSProtection(t *testing.T) {
	r := chi.NewRouter()
	r.Use(SecurityHeaders)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, "1; mode=block", rr.Header().Get("X-XSS-Protection"))
}

func TestSecurityHeaders_SetsReferrerPolicy(t *testing.T) {
	r := chi.NewRouter()
	r.Use(SecurityHeaders)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, "strict-origin-when-cross-origin", rr.Header().Get("Referrer-Policy"))
}

func TestSecurityHeaders_SetsPermissionsPolicy(t *testing.T) {
	r := chi.NewRouter()
	r.Use(SecurityHeaders)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, "geolocation=(), microphone=(), camera=()", rr.Header().Get("Permissions-Policy"))
}

func TestSecurityHeaders_SetsContentSecurityPolicy(t *testing.T) {
	r := chi.NewRouter()
	r.Use(SecurityHeaders)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	csp := rr.Header().Get("Content-Security-Policy")
	assert.NotEmpty(t, csp)
	assert.Contains(t, csp, "default-src 'self'")
	assert.Contains(t, csp, "script-src 'self'")
	assert.Contains(t, csp, "style-src 'self'")
	assert.Contains(t, csp, "img-src 'self' data: https:")
	assert.Contains(t, csp, "font-src 'self'")
	assert.Contains(t, csp, "connect-src 'self'")
	assert.Contains(t, csp, "frame-ancestors 'none'")
	assert.Contains(t, csp, "base-uri 'self'")
	assert.Contains(t, csp, "form-action 'self'")
}

// =============================================================================
// HSTS Tests
// =============================================================================

func TestSecurityHeaders_NoHSTS_HTTPRequest(t *testing.T) {
	r := chi.NewRouter()
	r.Use(SecurityHeaders)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// No TLS, no X-Forwarded-Proto header
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	// HSTS should NOT be set for HTTP
	assert.Empty(t, rr.Header().Get("Strict-Transport-Security"))
}

func TestSecurityHeaders_HSTS_TLSRequest(t *testing.T) {
	r := chi.NewRouter()
	r.Use(SecurityHeaders)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// Simulate TLS request
	req.TLS = &tls.ConnectionState{}
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	hsts := rr.Header().Get("Strict-Transport-Security")
	assert.NotEmpty(t, hsts)
	assert.Contains(t, hsts, "max-age=31536000")
	assert.Contains(t, hsts, "includeSubDomains")
	assert.Contains(t, hsts, "preload")
}

func TestSecurityHeaders_HSTS_XForwardedProtoHTTPS(t *testing.T) {
	r := chi.NewRouter()
	r.Use(SecurityHeaders)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	hsts := rr.Header().Get("Strict-Transport-Security")
	assert.NotEmpty(t, hsts)
	assert.Contains(t, hsts, "max-age=31536000")
}

func TestSecurityHeaders_NoHSTS_XForwardedProtoHTTP(t *testing.T) {
	r := chi.NewRouter()
	r.Use(SecurityHeaders)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-Proto", "http")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	// HSTS should NOT be set
	assert.Empty(t, rr.Header().Get("Strict-Transport-Security"))
}

// =============================================================================
// All Headers Present Tests
// =============================================================================

func TestSecurityHeaders_AllHeadersPresent(t *testing.T) {
	r := chi.NewRouter()
	r.Use(SecurityHeaders)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	expectedHeaders := []string{
		"X-Frame-Options",
		"X-Content-Type-Options",
		"X-XSS-Protection",
		"Referrer-Policy",
		"Permissions-Policy",
		"Content-Security-Policy",
	}

	for _, header := range expectedHeaders {
		assert.NotEmpty(t, rr.Header().Get(header), "Header %s should be present", header)
	}
}

func TestSecurityHeaders_AllHeadersPresentWithHTTPS(t *testing.T) {
	r := chi.NewRouter()
	r.Use(SecurityHeaders)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.TLS = &tls.ConnectionState{}
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	expectedHeaders := []string{
		"X-Frame-Options",
		"X-Content-Type-Options",
		"X-XSS-Protection",
		"Referrer-Policy",
		"Permissions-Policy",
		"Content-Security-Policy",
		"Strict-Transport-Security",
	}

	for _, header := range expectedHeaders {
		assert.NotEmpty(t, rr.Header().Get(header), "Header %s should be present", header)
	}
}

// =============================================================================
// Handler Execution Tests
// =============================================================================

func TestSecurityHeaders_HandlerExecuted(t *testing.T) {
	handlerCalled := false

	r := chi.NewRouter()
	r.Use(SecurityHeaders)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		_, _ = w.Write([]byte("OK"))
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.True(t, handlerCalled)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "OK", rr.Body.String())
}

func TestSecurityHeaders_PreservesHandlerHeaders(t *testing.T) {
	r := chi.NewRouter()
	r.Use(SecurityHeaders)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom-Header", "custom-value")
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	// Custom header should be preserved
	assert.Equal(t, "custom-value", rr.Header().Get("X-Custom-Header"))
	// Security headers should also be present
	assert.NotEmpty(t, rr.Header().Get("X-Frame-Options"))
}

// =============================================================================
// HTTP Methods Tests
// =============================================================================

func TestSecurityHeaders_AllHTTPMethods(t *testing.T) {
	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodOptions,
		http.MethodHead,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			r := chi.NewRouter()
			r.Use(SecurityHeaders)
			r.Method(method, "/test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(method, "/test", nil)
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			assert.Equal(t, "DENY", rr.Header().Get("X-Frame-Options"))
			assert.Equal(t, "nosniff", rr.Header().Get("X-Content-Type-Options"))
		})
	}
}

// =============================================================================
// Response Status Tests
// =============================================================================

func TestSecurityHeaders_DifferentResponseStatuses(t *testing.T) {
	statuses := []int{
		http.StatusOK,
		http.StatusCreated,
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusInternalServerError,
	}

	for _, status := range statuses {
		t.Run("", func(t *testing.T) {
			r := chi.NewRouter()
			r.Use(SecurityHeaders)
			r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			// Security headers should be present regardless of status
			assert.NotEmpty(t, rr.Header().Get("X-Frame-Options"))
			assert.NotEmpty(t, rr.Header().Get("X-Content-Type-Options"))
		})
	}
}

// =============================================================================
// CSP Directives Tests
// =============================================================================

func TestSecurityHeaders_CSP_DefaultSrc(t *testing.T) {
	r := chi.NewRouter()
	r.Use(SecurityHeaders)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	csp := rr.Header().Get("Content-Security-Policy")
	assert.Contains(t, csp, "default-src 'self'")
}

func TestSecurityHeaders_CSP_ScriptSrc(t *testing.T) {
	r := chi.NewRouter()
	r.Use(SecurityHeaders)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	csp := rr.Header().Get("Content-Security-Policy")
	assert.Contains(t, csp, "script-src 'self' 'unsafe-inline' 'unsafe-eval'")
}

func TestSecurityHeaders_CSP_ImgSrc(t *testing.T) {
	r := chi.NewRouter()
	r.Use(SecurityHeaders)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	csp := rr.Header().Get("Content-Security-Policy")
	assert.Contains(t, csp, "img-src 'self' data: https:")
}

func TestSecurityHeaders_CSP_FrameAncestors(t *testing.T) {
	r := chi.NewRouter()
	r.Use(SecurityHeaders)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	csp := rr.Header().Get("Content-Security-Policy")
	// frame-ancestors 'none' prevents embedding (clickjacking protection)
	assert.Contains(t, csp, "frame-ancestors 'none'")
}

// =============================================================================
// HSTS Max-Age Tests
// =============================================================================

func TestSecurityHeaders_HSTS_MaxAge(t *testing.T) {
	r := chi.NewRouter()
	r.Use(SecurityHeaders)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.TLS = &tls.ConnectionState{}
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	hsts := rr.Header().Get("Strict-Transport-Security")
	// max-age=31536000 = 1 year
	assert.Contains(t, hsts, "max-age=31536000")
}

// =============================================================================
// Middleware Chain Tests
// =============================================================================

func TestSecurityHeaders_WorksWithOtherMiddleware(t *testing.T) {
	customMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Custom-Middleware", "applied")
			next.ServeHTTP(w, r)
		})
	}

	r := chi.NewRouter()
	r.Use(customMiddleware)
	r.Use(SecurityHeaders)
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	// Both middleware should have been applied
	assert.Equal(t, "applied", rr.Header().Get("X-Custom-Middleware"))
	assert.NotEmpty(t, rr.Header().Get("X-Frame-Options"))
}
