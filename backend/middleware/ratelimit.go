package middleware

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/clientip"
	"golang.org/x/time/rate"
)

// RateLimiter manages rate limiting by request key.
//
// Counters are in-memory and per-process: each backend replica enforces the
// configured limit independently, so with N replicas the effective quota per
// key is N × limit. Phoenix currently runs a single backend process per
// environment; before scaling to multiple replicas, a shared store (e.g.
// Redis) is required if aggregate limits must hold (#2064).
type RateLimiter struct {
	visitors       map[string]*visitor
	mu             sync.RWMutex
	r              rate.Limit // requests per second
	b              int        // burst size
	ttl            time.Duration
	logger         *SecurityLogger // optional security logger
	keyFunc        func(*http.Request) string
	bucketFunc     func(*http.Request) string
	rejectObserver func(bucket string)
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

// SetBucketFunc partitions one identity's quota into independent buckets.
// The general API limiter uses this to keep SSE-driven reads from consuming
// the write budget. Auth limiters intentionally leave it unset.
func (rl *RateLimiter) SetBucketFunc(bucketFunc func(*http.Request) string) {
	rl.bucketFunc = bucketFunc
}

// SetRejectObserver records rejected requests without coupling this generic
// middleware to a metrics implementation.
func (rl *RateLimiter) SetRejectObserver(observer func(bucket string)) {
	rl.rejectObserver = observer
}

func defaultRateLimitKey(r *http.Request) string {
	return "ip:" + GetClientIP(r)
}

func (rl *RateLimiter) requestKey(r *http.Request) string {
	return rl.requestKeyForBucket(r, rl.requestBucket(r))
}

func (rl *RateLimiter) requestBucket(r *http.Request) string {
	if rl.bucketFunc == nil {
		return ""
	}
	return rl.bucketFunc(r)
}

func (rl *RateLimiter) requestKeyForBucket(r *http.Request, bucket string) string {
	key := defaultRateLimitKey(r)
	if rl.keyFunc == nil {
		// Keep the IP key.
	} else if customKey := rl.keyFunc(r); customKey != "" {
		key = customKey
	}
	if bucket != "" {
		return key + ":" + bucket
	}
	return key
}

func (rl *RateLimiter) retryAfterSeconds() int {
	if rl.r <= 0 {
		return 60
	}
	return max(1, int(math.Ceil(1/float64(rl.r))))
}

func (rl *RateLimiter) reject(w http.ResponseWriter, r *http.Request, bucket string) {
	retryAfter := rl.retryAfterSeconds()
	w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", int(rl.r*60)))
	w.Header().Set("X-RateLimit-Remaining", "0")
	w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(time.Duration(retryAfter)*time.Second).Unix()))
	w.Header().Set("Retry-After", strconv.Itoa(retryAfter))

	if rl.rejectObserver != nil {
		if bucket == "" {
			bucket = "shared"
		}
		rl.rejectObserver(bucket)
	}
	if rl.logger != nil {
		rl.logger.LogRateLimitExceeded(r)
	}
	http.Error(w, "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests)
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
			bucket := rl.requestBucket(r)
			limiter := rl.getVisitor(rl.requestKeyForBucket(r, bucket))
			if !limiter.Allow() {
				rl.reject(w, r, bucket)
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
