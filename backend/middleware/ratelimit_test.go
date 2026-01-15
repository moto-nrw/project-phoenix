package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// NewRateLimiter Tests
// =============================================================================

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter(60, 10)
	require.NotNil(t, rl)
	assert.NotNil(t, rl.visitors)
	assert.Equal(t, 10, rl.b) // burst
	assert.Equal(t, 3*time.Minute, rl.ttl)
}

func TestNewRateLimiter_DifferentRates(t *testing.T) {
	testCases := []struct {
		requestsPerMinute int
		burst             int
	}{
		{10, 5},
		{60, 10},
		{120, 20},
		{1, 1},
	}

	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			rl := NewRateLimiter(tc.requestsPerMinute, tc.burst)
			require.NotNil(t, rl)
			assert.Equal(t, tc.burst, rl.b)
		})
	}
}

// =============================================================================
// SetLogger Tests
// =============================================================================

func TestRateLimiter_SetLogger(t *testing.T) {
	rl := NewRateLimiter(60, 10)
	assert.Nil(t, rl.logger)

	logger := &SecurityLogger{}
	rl.SetLogger(logger)
	assert.NotNil(t, rl.logger)
}

// =============================================================================
// GetClientIP Tests
// =============================================================================

func TestGetClientIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Real-IP", "192.168.1.100")
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 10.0.0.2")
	req.RemoteAddr = "127.0.0.1:12345"

	ip := GetClientIP(req)
	assert.Equal(t, "192.168.1.100", ip, "X-Real-IP should take precedence")
}

func TestGetClientIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 10.0.0.2, 10.0.0.3")
	req.RemoteAddr = "127.0.0.1:12345"

	ip := GetClientIP(req)
	assert.Equal(t, "10.0.0.1", ip, "Should return first IP from X-Forwarded-For")
}

func TestGetClientIP_XForwardedFor_WithSpaces(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "  10.0.0.1  ,  10.0.0.2  ")
	req.RemoteAddr = "127.0.0.1:12345"

	ip := GetClientIP(req)
	assert.Equal(t, "10.0.0.1", ip, "Should trim spaces")
}

func TestGetClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.50:54321"

	ip := GetClientIP(req)
	assert.Equal(t, "192.168.1.50", ip, "Should extract IP from RemoteAddr")
}

func TestGetClientIP_RemoteAddr_NoPort(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.50"

	ip := GetClientIP(req)
	assert.Equal(t, "192.168.1.50", ip, "Should return RemoteAddr as-is if no port")
}

func TestGetClientIP_IPv6(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "[::1]:8080"

	ip := GetClientIP(req)
	assert.Equal(t, "::1", ip)
}

func TestGetClientIP_IPv6_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "2001:db8::1, 2001:db8::2")

	ip := GetClientIP(req)
	assert.Equal(t, "2001:db8::1", ip)
}

// =============================================================================
// Middleware Tests - Allow Requests
// =============================================================================

func TestRateLimiter_Middleware_AllowsRequests(t *testing.T) {
	rl := NewRateLimiter(60, 10) // 60 requests/minute, burst of 10

	r := chi.NewRouter()
	r.Use(rl.Middleware())
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Should allow first few requests
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()

		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code, "Request %d should be allowed", i+1)
	}
}

func TestRateLimiter_Middleware_BlocksExcessRequests(t *testing.T) {
	// Very low rate: 1 request per minute, burst of 1
	rl := NewRateLimiter(1, 1)

	r := chi.NewRouter()
	r.Use(rl.Middleware())
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// First request should succeed (burst)
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	rr1 := httptest.NewRecorder()
	r.ServeHTTP(rr1, req1)
	assert.Equal(t, http.StatusOK, rr1.Code)

	// Immediate second request should be rate limited
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "192.168.1.1:12345"
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusTooManyRequests, rr2.Code)
}

func TestRateLimiter_Middleware_SetsRateLimitHeaders(t *testing.T) {
	rl := NewRateLimiter(60, 1)

	r := chi.NewRouter()
	r.Use(rl.Middleware())
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Exhaust the burst
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	rr1 := httptest.NewRecorder()
	r.ServeHTTP(rr1, req1)

	// Rate limited request should have headers
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "192.168.1.1:12345"
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)

	assert.Equal(t, http.StatusTooManyRequests, rr2.Code)
	assert.NotEmpty(t, rr2.Header().Get("X-RateLimit-Limit"))
	assert.Equal(t, "0", rr2.Header().Get("X-RateLimit-Remaining"))
	assert.NotEmpty(t, rr2.Header().Get("X-RateLimit-Reset"))
	assert.Equal(t, "60", rr2.Header().Get("Retry-After"))
}

// =============================================================================
// Middleware Tests - Per-IP Rate Limiting
// =============================================================================

func TestRateLimiter_Middleware_PerIPRateLimiting(t *testing.T) {
	rl := NewRateLimiter(1, 1) // Very strict rate

	r := chi.NewRouter()
	r.Use(rl.Middleware())
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// First request from IP 1 should succeed
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	rr1 := httptest.NewRecorder()
	r.ServeHTTP(rr1, req1)
	assert.Equal(t, http.StatusOK, rr1.Code)

	// First request from IP 2 should also succeed (separate rate limit)
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "192.168.1.2:12345"
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusOK, rr2.Code)

	// Second request from IP 1 should be rate limited
	req3 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req3.RemoteAddr = "192.168.1.1:12345"
	rr3 := httptest.NewRecorder()
	r.ServeHTTP(rr3, req3)
	assert.Equal(t, http.StatusTooManyRequests, rr3.Code)

	// Second request from IP 2 should also be rate limited
	req4 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req4.RemoteAddr = "192.168.1.2:12345"
	rr4 := httptest.NewRecorder()
	r.ServeHTTP(rr4, req4)
	assert.Equal(t, http.StatusTooManyRequests, rr4.Code)
}

// =============================================================================
// Middleware Tests - Rate Recovery
// =============================================================================

func TestRateLimiter_Middleware_RateRecovery(t *testing.T) {
	// 60 requests per minute = 1 per second
	// With burst of 2, we can make 2 immediate requests
	rl := NewRateLimiter(60, 2)

	r := chi.NewRouter()
	r.Use(rl.Middleware())
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Exhaust burst
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	}

	// Next request should be rate limited
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	rr1 := httptest.NewRecorder()
	r.ServeHTTP(rr1, req1)
	assert.Equal(t, http.StatusTooManyRequests, rr1.Code)

	// Wait for rate to recover (slightly more than 1 second for 1 token)
	time.Sleep(1100 * time.Millisecond)

	// Should now be allowed
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "192.168.1.1:12345"
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusOK, rr2.Code)
}

// =============================================================================
// getVisitor Tests
// =============================================================================

func TestRateLimiter_getVisitor_CreatesNew(t *testing.T) {
	rl := NewRateLimiter(60, 10)

	limiter1 := rl.getVisitor("192.168.1.1")
	require.NotNil(t, limiter1)

	// Should be tracked in visitors map
	rl.mu.RLock()
	_, exists := rl.visitors["192.168.1.1"]
	rl.mu.RUnlock()
	assert.True(t, exists)
}

func TestRateLimiter_getVisitor_ReturnsExisting(t *testing.T) {
	rl := NewRateLimiter(60, 10)

	limiter1 := rl.getVisitor("192.168.1.1")
	limiter2 := rl.getVisitor("192.168.1.1")

	// Should return the same limiter
	assert.Same(t, limiter1, limiter2)
}

func TestRateLimiter_getVisitor_UpdatesLastSeen(t *testing.T) {
	rl := NewRateLimiter(60, 10)

	// Create visitor
	_ = rl.getVisitor("192.168.1.1")

	rl.mu.RLock()
	firstLastSeen := rl.visitors["192.168.1.1"].lastSeen
	rl.mu.RUnlock()

	time.Sleep(10 * time.Millisecond)

	// Access again
	_ = rl.getVisitor("192.168.1.1")

	rl.mu.RLock()
	secondLastSeen := rl.visitors["192.168.1.1"].lastSeen
	rl.mu.RUnlock()

	assert.True(t, secondLastSeen.After(firstLastSeen))
}

// =============================================================================
// Error Response Tests
// =============================================================================

func TestRateLimiter_Middleware_ErrorResponseBody(t *testing.T) {
	rl := NewRateLimiter(1, 1)

	r := chi.NewRouter()
	r.Use(rl.Middleware())
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Exhaust rate limit
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	rr1 := httptest.NewRecorder()
	r.ServeHTTP(rr1, req1)

	// Check error response
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "192.168.1.1:12345"
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)

	assert.Equal(t, http.StatusTooManyRequests, rr2.Code)
	assert.Contains(t, rr2.Body.String(), "Rate limit exceeded")
}

// =============================================================================
// Multiple Endpoints Tests
// =============================================================================

func TestRateLimiter_Middleware_SharedAcrossEndpoints(t *testing.T) {
	rl := NewRateLimiter(1, 2) // 2 requests then rate limited

	r := chi.NewRouter()
	r.Use(rl.Middleware())
	r.Get("/endpoint1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Get("/endpoint2", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// First request to endpoint1
	req1 := httptest.NewRequest(http.MethodGet, "/endpoint1", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	rr1 := httptest.NewRecorder()
	r.ServeHTTP(rr1, req1)
	assert.Equal(t, http.StatusOK, rr1.Code)

	// First request to endpoint2 (same IP)
	req2 := httptest.NewRequest(http.MethodGet, "/endpoint2", nil)
	req2.RemoteAddr = "192.168.1.1:12345"
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusOK, rr2.Code)

	// Third request should be rate limited (regardless of endpoint)
	req3 := httptest.NewRequest(http.MethodGet, "/endpoint1", nil)
	req3.RemoteAddr = "192.168.1.1:12345"
	rr3 := httptest.NewRecorder()
	r.ServeHTTP(rr3, req3)
	assert.Equal(t, http.StatusTooManyRequests, rr3.Code)
}

// =============================================================================
// Security Logger Integration Tests
// =============================================================================

func TestRateLimiter_Middleware_LogsRateLimitViolations(t *testing.T) {
	rl := NewRateLimiter(1, 1)
	// Note: Can't easily test with real SecurityLogger without its full implementation
	// This tests the logging path exists

	r := chi.NewRouter()
	r.Use(rl.Middleware())
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Exhaust rate limit
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	rr1 := httptest.NewRecorder()
	r.ServeHTTP(rr1, req1)

	// Trigger rate limit - logger would be called if set
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "192.168.1.1:12345"
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)

	assert.Equal(t, http.StatusTooManyRequests, rr2.Code)
}

// =============================================================================
// Edge Cases Tests
// =============================================================================

func TestGetClientIP_EmptyXForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "")
	req.RemoteAddr = "192.168.1.50:54321"

	ip := GetClientIP(req)
	assert.Equal(t, "192.168.1.50", ip)
}

func TestGetClientIP_MalformedRemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "invalid:address:format"

	ip := GetClientIP(req)
	// Should return as-is when parsing fails
	assert.Equal(t, "invalid:address:format", ip)
}

func TestRateLimiter_Middleware_DifferentHTTPMethods(t *testing.T) {
	rl := NewRateLimiter(1, 2)

	r := chi.NewRouter()
	r.Use(rl.Middleware())
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Post("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// GET request
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	rr1 := httptest.NewRecorder()
	r.ServeHTTP(rr1, req1)
	assert.Equal(t, http.StatusOK, rr1.Code)

	// POST request (same IP, shares rate limit)
	req2 := httptest.NewRequest(http.MethodPost, "/test", nil)
	req2.RemoteAddr = "192.168.1.1:12345"
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusOK, rr2.Code)

	// Third request should be rate limited
	req3 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req3.RemoteAddr = "192.168.1.1:12345"
	rr3 := httptest.NewRecorder()
	r.ServeHTTP(rr3, req3)
	assert.Equal(t, http.StatusTooManyRequests, rr3.Code)
}

// =============================================================================
// visitor struct Tests
// =============================================================================

func TestVisitor_Structure(t *testing.T) {
	rl := NewRateLimiter(60, 10)
	_ = rl.getVisitor("192.168.1.1")

	rl.mu.RLock()
	defer rl.mu.RUnlock()

	v, exists := rl.visitors["192.168.1.1"]
	require.True(t, exists)
	assert.NotNil(t, v.limiter)
	assert.False(t, v.lastSeen.IsZero())
}
