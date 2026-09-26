package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
