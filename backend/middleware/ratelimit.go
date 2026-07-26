package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/clientip"
	"golang.org/x/time/rate"
)

// RateLimiter manages rate limiting by request key.
type RateLimiter struct {
	visitors map[string]*visitor
	mu       sync.RWMutex
	r        rate.Limit // requests per second
	b        int        // burst size
	ttl      time.Duration
	logger   *SecurityLogger // optional security logger
	keyFunc  func(*http.Request) string
}

// visitor tracks rate limiting for a single request key.
type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewRateLimiter creates a new RateLimiter
// requestsPerMinute: number of requests allowed per minute
// burst: number of requests allowed in a burst
func NewRateLimiter(requestsPerMinute, burst int) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		r:        rate.Limit(float64(requestsPerMinute) / 60.0), // convert to per second
		b:        burst,
		ttl:      3 * time.Minute, // cleanup visitors after 3 minutes of inactivity
		logger:   nil,             // can be set with SetLogger
	}

	// Start cleanup goroutine
	go rl.cleanupVisitors()

	return rl
}

// SetLogger sets the security logger for the rate limiter
func (rl *RateLimiter) SetLogger(logger *SecurityLogger) {
	rl.logger = logger
}

// SetKeyFunc sets the request key function for the rate limiter.
// If keyFunc returns an empty key, the limiter falls back to the client IP.
func (rl *RateLimiter) SetKeyFunc(keyFunc func(*http.Request) string) {
	rl.keyFunc = keyFunc
}

func defaultRateLimitKey(r *http.Request) string {
	return "ip:" + GetClientIP(r)
}

func (rl *RateLimiter) requestKey(r *http.Request) string {
	if rl.keyFunc == nil {
		return defaultRateLimitKey(r)
	}
	key := rl.keyFunc(r)
	if key == "" {
		return defaultRateLimitKey(r)
	}
	return key
}

// getVisitor returns the rate limiter for the given request key.
func (rl *RateLimiter) getVisitor(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[key]
	if !exists {
		limiter := rate.NewLimiter(rl.r, rl.b)
		rl.visitors[key] = &visitor{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}

	// Update last seen time
	v.lastSeen = time.Now()
	return v.limiter
}

// cleanupVisitors removes old entries from the visitors map
func (rl *RateLimiter) cleanupVisitors() {
	for {
		time.Sleep(time.Minute)

		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > rl.ttl {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// Middleware returns the rate limiting middleware
func (rl *RateLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			limiter := rl.getVisitor(rl.requestKey(r))

			if !limiter.Allow() {
				w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", int(rl.r*60)))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(time.Minute).Unix()))
				w.Header().Set("Retry-After", "60")

				// Log rate limit violation if logger is available
				if rl.logger != nil {
					rl.logger.LogRateLimitExceeded(r)
				}

				http.Error(w, "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GetClientIP extracts the real client IP address from the request
func GetClientIP(r *http.Request) string {
	return clientip.GetClientIPString(r)
}
