package middleware

import (
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
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
	exemptPrefixes []string
	exemptRequests []func(*http.Request) bool
	exemptLoopback bool
	refusalsOnly   bool
	alwaysSpends   func(*http.Request) bool
}

// visitor tracks rate limiting for a single request key. pending counts the
// requests of a failures-only limiter that hold a token while their handler
// runs.
type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
	pending  int
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
// the write budget. Auth and webhook limiters use a fixed bucket name for
// metrics; their instances still have independent quotas.
func (rl *RateLimiter) SetBucketFunc(bucketFunc func(*http.Request) string) {
	rl.bucketFunc = bucketFunc
}

// SetRejectObserver records rejected requests without coupling this generic
// middleware to a metrics implementation.
func (rl *RateLimiter) SetRejectObserver(observer func(bucket string)) {
	rl.rejectObserver = observer
}

// ExemptLoopback lets requests from a loopback TCP peer bypass the limiter.
// The public demo enables it for its demo process, which shares the server
// container's network namespace and calls the API on 127.0.0.1 while it
// seeds and ticks demo schools. Everything else keeps its limits.
func (rl *RateLimiter) ExemptLoopback() {
	rl.exemptLoopback = true
}

// SpendOnRefusals makes the limiter spend a token only on a refused request
// (a 4xx answer of the handler) and on a request alwaysSpends reports, such
// as one that sends a mail; nil reports none. An accepted or failed (5xx)
// request gives its token back.
//
// A request needs a free token to enter and holds it while its handler runs,
// so parallel requests cannot pass together beyond the burst: at most burst
// requests of one key run at once, however many of them succeed.
//
// The login limiter uses it (#6738); docs/agents/contracts.md, "Client IP and
// rate limits", explains why.
func (rl *RateLimiter) SpendOnRefusals(alwaysSpends func(*http.Request) bool) {
	rl.refusalsOnly = true
	rl.alwaysSpends = alwaysSpends
}

// ExemptPathPrefix lets a route that has its own limiter bypass this one.
func (rl *RateLimiter) ExemptPathPrefix(prefix string) {
	rl.exemptPrefixes = append(rl.exemptPrefixes, prefix)
}

// ExemptRequest lets the requests match selects bypass this limiter, for
// routes with their own limiter that no path prefix describes.
func (rl *RateLimiter) ExemptRequest(match func(*http.Request) bool) {
	rl.exemptRequests = append(rl.exemptRequests, match)
}

func (rl *RateLimiter) exemptPath(r *http.Request) bool {
	for _, prefix := range rl.exemptPrefixes {
		if strings.HasPrefix(r.URL.Path, prefix) {
			return true
		}
	}
	for _, match := range rl.exemptRequests {
		if match(r) {
			return true
		}
	}
	return false
}

// isLoopbackPeer reports whether the request's TCP peer is a loopback address.
//
// The root router rewrites RemoteAddr from the rightmost X-Forwarded-For entry
// (ClientIPFromXFFTrustedProxies and syncClientIPToRemoteAddr in api/serve_middleware.go), so a request
// that carries the header has a RemoteAddr chosen by a header, not by the
// connection. Only a request without the header still holds the TCP peer, and
// only such a request can be exempt.
//
// Public traffic never qualifies: Caddy on the host sets X-Forwarded-For and
// reaches the container through Docker's published port, where the peer is
// the bridge gateway (for example 172.18.0.1), never loopback. The container's
// loopback is reachable only from inside the server's network namespace.
func isLoopbackPeer(r *http.Request) bool {
	if len(r.Header.Values("X-Forwarded-For")) > 0 {
		return false
	}
	ip := net.ParseIP(GetClientIP(r))
	return ip != nil && ip.IsLoopback()
}

func defaultRateLimitKey(r *http.Request) string {
	return "ip:" + GetClientIP(r)
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
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(rateLimitProblem{
		Status: http.StatusTooManyRequests,
		Detail: "Rate limit exceeded. Please try again later.",
		Code:   codeGeneralUnavailable,
	})
}

// codeGeneralUnavailable is the registered class code (error-registry/)
// of a rate-limited request. This package may not import api/common, so it
// names the code once here.
const codeGeneralUnavailable = "general.unavailable"

// rateLimitProblem is the shared error envelope (ADR 0006) of a rejected
// request: the HTTP status number, the diagnostic text and the registered
// code. ProblemResponseMiddleware adds type, title and instance like for
// every error answer.
type rateLimitProblem struct {
	Status int    `json:"status"`
	Detail string `json:"detail"`
	Code   string `json:"code"`
}

// visitorLocked returns the visitor of the request key and marks it seen.
// The caller holds rl.mu.
func (rl *RateLimiter) visitorLocked(key string) *visitor {
	v, exists := rl.visitors[key]
	if !exists {
		v = &visitor{limiter: rate.NewLimiter(rl.r, rl.b)}
		rl.visitors[key] = v
	}
	v.lastSeen = time.Now()
	return v
}

// getVisitor returns the rate limiter for the given request key.
func (rl *RateLimiter) getVisitor(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return rl.visitorLocked(key).limiter
}

// admitPending lets a request of a failures-only limiter in while a token is
// free beside the ones running requests hold, and holds one for it.
func (rl *RateLimiter) admitPending(key string) (*visitor, bool) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	v := rl.visitorLocked(key)
	if v.limiter.Tokens() < float64(v.pending+1) {
		return nil, false
	}
	v.pending++
	return v, true
}

// settlePending releases the token a request held, or spends it.
func (rl *RateLimiter) settlePending(v *visitor, spend bool) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	v.pending--
	v.lastSeen = time.Now()
	if spend {
		v.limiter.Allow()
	}
}

// serveCountingFailures runs a request through a failures-only limiter.
func (rl *RateLimiter) serveCountingFailures(w http.ResponseWriter, r *http.Request, next http.Handler, bucket string) {
	v, ok := rl.admitPending(rl.requestKeyForBucket(r, bucket))
	if !ok {
		rl.reject(w, r, bucket)
		return
	}
	alwaysSpends := rl.alwaysSpends != nil && rl.alwaysSpends(r)
	ww := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
	defer func() {
		status := ww.Status()
		rl.settlePending(v, alwaysSpends || (status >= http.StatusBadRequest && status < http.StatusInternalServerError))
	}()
	next.ServeHTTP(ww, r)
}

// cleanupVisitors removes old entries from the visitors map
func (rl *RateLimiter) cleanupVisitors() {
	for {
		time.Sleep(time.Minute)

		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if v.pending == 0 && time.Since(v.lastSeen) > rl.ttl {
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
			if (rl.exemptLoopback && isLoopbackPeer(r)) || rl.exemptPath(r) {
				next.ServeHTTP(w, r)
				return
			}
			bucket := rl.requestBucket(r)
			if rl.refusalsOnly {
				rl.serveCountingFailures(w, r, next, bucket)
				return
			}
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
	if r == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
