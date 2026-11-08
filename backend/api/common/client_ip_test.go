package common

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/assert"
)

// The client IP is recorded in audit rows and bound into the trusted-device
// cookie's HMAC payload, so its header precedence is pinned here.

func TestGetClientIP_IgnoresRawXRealIP(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Real-IP", "192.168.1.100")
	req.RemoteAddr = "192.0.2.50:1234"

	ip := GetClientIPString(req)

	assert.Equal(t, "192.0.2.50", ip)
}

func TestGetClientIP_XForwardedFor_Single(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.50")

	ip := clientIPThroughXFFMiddleware(req)

	assert.Equal(t, "10.0.0.50", ip)
}

func TestGetClientIP_XForwardedFor_Multiple(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 10.0.0.2, 10.0.0.3")

	ip := clientIPThroughXFFMiddleware(req)

	assert.Equal(t, "10.0.0.3", ip)
}

func TestGetClientIP_XForwardedFor_WithSpaces(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-For", "  10.0.0.1  ,  10.0.0.2  ")

	ip := clientIPThroughXFFMiddleware(req)

	assert.Equal(t, "10.0.0.2", ip)
}

func TestGetClientIP_RemoteAddr_WithPort(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	ip := GetClientIPString(req)

	assert.Equal(t, "192.168.1.1", ip)
}

func TestGetClientIP_RemoteAddr_IPv6(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "[::1]:8080"

	ip := GetClientIPString(req)

	assert.Equal(t, "::1", ip)
}

func TestGetClientIP_RemoteAddr_NoPort(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1"

	ip := GetClientIPString(req)

	// If SplitHostPort fails, returns the full RemoteAddr
	assert.Equal(t, "192.168.1.1", ip)
}

func TestGetClientIP_XForwardedForBeatsRawXRealIP(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Real-IP", "1.1.1.1")
	req.Header.Set("X-Forwarded-For", "2.2.2.2")
	req.RemoteAddr = "3.3.3.3:1234"

	ip := clientIPThroughXFFMiddleware(req)

	assert.Equal(t, "2.2.2.2", ip)
}

func TestGetClientIP_XForwardedFor_TakesPrecedence_OverRemoteAddr(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-For", "2.2.2.2")
	req.RemoteAddr = "3.3.3.3:1234"

	ip := clientIPThroughXFFMiddleware(req)

	assert.Equal(t, "2.2.2.2", ip)
}

func TestGetClientIP_MalformedXForwardedForFallsBackToRemoteAddr(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.5, not-an-ip")
	req.RemoteAddr = "10.0.0.1:54321"

	assert.Equal(t, "10.0.0.1", clientIPThroughXFFMiddleware(req))
}

func TestGetClientIP_NoHeadersNoAddrReturnsEmpty(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = ""

	assert.Equal(t, "", GetClientIPString(req))
}

func clientIPThroughXFFMiddleware(req *http.Request) string {
	var ip string
	router := chi.NewRouter()
	router.Use(chimiddleware.ClientIPFromXFFTrustedProxies(1))
	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		ip = GetClientIPString(r)
		w.WriteHeader(http.StatusNoContent)
	})
	router.ServeHTTP(httptest.NewRecorder(), req)
	return ip
}

func TestParseClientIP_UsesChiXFFOverRawXRealIP(t *testing.T) {
	t.Parallel()

	r := httptest.NewRequest(http.MethodGet, "/mfa", nil)
	r.Header.Set("X-Real-IP", "203.0.113.10")
	r.Header.Set("X-Forwarded-For", "198.51.100.7")
	r.RemoteAddr = "10.0.0.1:54321"

	ip := parsedClientIPThroughXFFMiddleware(r)
	if assert.NotNil(t, ip) {
		assert.Equal(t, "198.51.100.7", ip.String())
	}
}

func TestParseClientIP_UsesRightmostXForwardedForHop(t *testing.T) {
	t.Parallel()

	r := httptest.NewRequest(http.MethodGet, "/mfa", nil)
	r.Header.Set("X-Forwarded-For", "198.51.100.7, 10.0.0.1, 172.16.0.1")
	r.RemoteAddr = "10.0.0.1:54321"

	ip := parsedClientIPThroughXFFMiddleware(r)
	if assert.NotNil(t, ip) {
		assert.Equal(t, "172.16.0.1", ip.String())
	}
}

func TestParseClientIP_FallsBackToRemoteAddr(t *testing.T) {
	t.Parallel()

	r := httptest.NewRequest(http.MethodGet, "/mfa", nil)
	r.RemoteAddr = "192.0.2.5:54321"

	ip := ParseClientIP(r)
	if assert.NotNil(t, ip) {
		assert.Equal(t, "192.0.2.5", ip.String())
	}
}

func TestParseClientIP_ParsesIPv6(t *testing.T) {
	t.Parallel()

	r := httptest.NewRequest(http.MethodGet, "/mfa", nil)
	r.Header.Set("X-Forwarded-For", "2001:db8::1")

	ip := parsedClientIPThroughXFFMiddleware(r)
	if assert.NotNil(t, ip) {
		assert.Equal(t, "2001:db8::1", ip.String())
	}
}

func TestParseClientIP_ReturnsNilOnUnparseable(t *testing.T) {
	t.Parallel()

	r := httptest.NewRequest(http.MethodGet, "/mfa", nil)
	r.Header.Set("X-Forwarded-For", "not-an-ip")
	r.RemoteAddr = ""

	assert.Nil(t, parsedClientIPThroughXFFMiddleware(r),
		"unparseable IP strings must yield nil so callers don't bind garbage into the HMAC payload")
}

func TestParseClientIP_ReturnsNilOnEmptyRequest(t *testing.T) {
	t.Parallel()

	r := httptest.NewRequest(http.MethodGet, "/mfa", nil)
	r.RemoteAddr = ""

	assert.Nil(t, ParseClientIP(r),
		"no header + empty RemoteAddr must produce a nil IP")
}

func parsedClientIPThroughXFFMiddleware(req *http.Request) net.IP {
	var ip net.IP
	router := chi.NewRouter()
	router.Use(chimiddleware.ClientIPFromXFFTrustedProxies(1))
	router.Get("/mfa", func(w http.ResponseWriter, r *http.Request) {
		ip = ParseClientIP(r)
		w.WriteHeader(http.StatusNoContent)
	})
	router.ServeHTTP(httptest.NewRecorder(), req)
	return ip
}
