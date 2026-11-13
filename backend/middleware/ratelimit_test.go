package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// statusesFor sends the same request twice through a limiter with a burst of
// one and returns both response codes.
func statusesFor(exemptLoopback bool, remoteAddr string, headers map[string]string) [2]int {
	limiter := NewRateLimiter(1, 1)
	if exemptLoopback {
		limiter.ExemptLoopback()
	}
	handler := limiter.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	var codes [2]int
	for i := range codes {
		req := httptest.NewRequest(http.MethodPost, "/api/iot/checkin", nil)
		req.RemoteAddr = remoteAddr
		for name, value := range headers {
			req.Header.Set(name, value)
		}
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		codes[i] = rr.Code
	}
	return codes
}

func TestRateLimiterLoopbackExemption(t *testing.T) {
	t.Parallel()

	limited := [2]int{http.StatusNoContent, http.StatusTooManyRequests}
	passed := [2]int{http.StatusNoContent, http.StatusNoContent}
	for _, tc := range []struct {
		name       string
		exempt     bool
		remoteAddr string
		headers    map[string]string
		want       [2]int
	}{
		{name: "loopback peer is exempt with the option", exempt: true, remoteAddr: "127.0.0.1:54321", want: passed},
		{name: "IPv6 loopback peer is exempt with the option", exempt: true, remoteAddr: "[::1]:54321", want: passed},
		{name: "loopback peer is limited without the option", exempt: false, remoteAddr: "127.0.0.1:54321", want: limited},
		{name: "docker gateway peer is limited with the option", exempt: true, remoteAddr: "172.18.0.1:54321", want: limited},
		{name: "public peer is limited with the option", exempt: true, remoteAddr: "203.0.113.10:54321", want: limited},
		// The root router rewrites RemoteAddr from X-Forwarded-For, so a
		// loopback address next to the header proves nothing about the peer.
		{
			name: "forwarded request with a loopback address is limited", exempt: true,
			remoteAddr: "127.0.0.1", headers: map[string]string{"X-Forwarded-For": "127.0.0.1"}, want: limited,
		},
		{
			name: "loopback peer that forwards a client is limited", exempt: true,
			remoteAddr: "127.0.0.1:54321", headers: map[string]string{"X-Forwarded-For": "203.0.113.10"}, want: limited,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := statusesFor(tc.exempt, tc.remoteAddr, tc.headers); got != tc.want {
				t.Fatalf("status codes = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRateLimiterExemptPathPrefixBypassesOnlyThatPath(t *testing.T) {
	t.Parallel()
	limiter := NewRateLimiter(1, 1)
	limiter.ExemptPathPrefix("/webhooks/")
	handler := limiter.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	status := func(path string) int {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, path, nil))
		return recorder.Code
	}

	for range 5 {
		assert.Equal(t, http.StatusNoContent, status("/webhooks/email/delivery/lettermint"))
	}
	assert.Equal(t, http.StatusNoContent, status("/api/x"))
	assert.Equal(t, http.StatusTooManyRequests, status("/api/x"), "other paths keep the limit")
}

// A failures-only limiter lets any number of accepted requests from one
// address through and refuses only once its refused requests used the burst
// (#6738: parents of one school network signing in together).
func TestRateLimiterSpendOnRefusalsSpendsTokensOnRefusals(t *testing.T) {
	t.Parallel()
	limiter := NewRateLimiter(1, 3)
	limiter.SpendOnRefusals(nil)
	handler := limiter.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("password") {
		case "wrong":
			w.WriteHeader(http.StatusUnauthorized)
		case "broken":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	status := func(password string) int {
		req := httptest.NewRequest(http.MethodPost, "/auth/login?password="+password, nil)
		req.RemoteAddr = "203.0.113.7:54321"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr.Code
	}

	for range 60 {
		assert.Equal(t, http.StatusOK, status("right"), "an accepted login spends no token")
		assert.Equal(t, http.StatusInternalServerError, status("broken"), "a server error is no refusal")
	}
	for range 3 {
		assert.Equal(t, http.StatusUnauthorized, status("wrong"))
	}
	assert.Equal(t, http.StatusTooManyRequests, status("right"), "the refusals used the burst")
	assert.Equal(t, http.StatusTooManyRequests, status("wrong"))
}

// Parallel requests hold their token while they run, so a burst of parallel
// guesses cannot pass the limiter together before the first is refused.
func TestRateLimiterSpendOnRefusalsHoldsTokensOfRunningRequests(t *testing.T) {
	t.Parallel()
	limiter := NewRateLimiter(1, 2)
	limiter.SpendOnRefusals(nil)
	entered := make(chan struct{})
	release := make(chan struct{})
	handler := limiter.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Has("hold") {
			entered <- struct{}{}
			<-release
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	serve := func(path string) int {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.RemoteAddr = "203.0.113.7:54321"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr.Code
	}

	results := make(chan int, 2)
	for range 2 {
		go func() { results <- serve("/auth/login?hold") }()
		select {
		case <-entered:
		case code := <-results:
			t.Fatalf("a held request answered %d before its handler ran", code)
		}
	}
	assert.Equal(t, http.StatusTooManyRequests, serve("/auth/login"), "both tokens are held by running requests")
	close(release)
	for range 2 {
		assert.Equal(t, http.StatusNoContent, <-results)
	}
	assert.Equal(t, http.StatusNoContent, serve("/auth/login"), "accepted requests gave their tokens back")
}

// A request alwaysSpends reports spends its token even when accepted: a mail
// a login route sends keeps the limit of one request per token.
func TestRateLimiterSpendOnRefusalsSpendsAlwaysOnMarkedRequests(t *testing.T) {
	t.Parallel()
	limiter := NewRateLimiter(1, 2)
	limiter.SpendOnRefusals(func(r *http.Request) bool { return r.URL.Path == "/auth/password-reset" })
	handler := limiter.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	status := func(path string) int {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.RemoteAddr = "203.0.113.7:54321"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr.Code
	}

	for range 5 {
		assert.Equal(t, http.StatusAccepted, status("/auth/login"))
	}
	assert.Equal(t, http.StatusAccepted, status("/auth/password-reset"))
	assert.Equal(t, http.StatusAccepted, status("/auth/password-reset"))
	assert.Equal(t, http.StatusTooManyRequests, status("/auth/password-reset"))
	assert.Equal(t, http.StatusTooManyRequests, status("/auth/login"), "the address has no token left")
}
