package auth

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/assert"
)

// parseClientIP feeds the IP that gets bound into the trusted-device cookie's
// HMAC payload — it's tiny, deterministic, and easy to break in a future
// header-precedence refactor, so we lock its behaviour down here. (The
// matching Secure flag is now a literal `true`, so no helper test is needed.)

func TestParseClientIP_UsesChiXFFOverRawXRealIP(t *testing.T) {
	t.Parallel()

	r := httptest.NewRequest(http.MethodGet, "/mfa", nil)
	r.Header.Set("X-Real-IP", "203.0.113.10")
	r.Header.Set("X-Forwarded-For", "198.51.100.7")
	r.RemoteAddr = "10.0.0.1:54321"

	ip := parseClientIPThroughXFFMiddleware(r)
	if assert.NotNil(t, ip) {
		assert.Equal(t, "198.51.100.7", ip.String())
	}
}

func TestParseClientIP_UsesRightmostXForwardedForHop(t *testing.T) {
	t.Parallel()

	r := httptest.NewRequest(http.MethodGet, "/mfa", nil)
	r.Header.Set("X-Forwarded-For", "198.51.100.7, 10.0.0.1, 172.16.0.1")
	r.RemoteAddr = "10.0.0.1:54321"

	ip := parseClientIPThroughXFFMiddleware(r)
	if assert.NotNil(t, ip) {
		assert.Equal(t, "172.16.0.1", ip.String())
	}
}

func TestParseClientIP_FallsBackToRemoteAddr(t *testing.T) {
	t.Parallel()

	r := httptest.NewRequest(http.MethodGet, "/mfa", nil)
	r.RemoteAddr = "192.0.2.5:54321"

	ip := parseClientIP(r)
	if assert.NotNil(t, ip) {
		assert.Equal(t, "192.0.2.5", ip.String())
	}
}

func TestParseClientIP_ParsesIPv6(t *testing.T) {
	t.Parallel()

	r := httptest.NewRequest(http.MethodGet, "/mfa", nil)
	r.Header.Set("X-Forwarded-For", "2001:db8::1")

	ip := parseClientIPThroughXFFMiddleware(r)
	if assert.NotNil(t, ip) {
		assert.Equal(t, "2001:db8::1", ip.String())
	}
}

func TestParseClientIP_ReturnsNilOnUnparseable(t *testing.T) {
	t.Parallel()

	r := httptest.NewRequest(http.MethodGet, "/mfa", nil)
	r.Header.Set("X-Forwarded-For", "not-an-ip")
	r.RemoteAddr = ""

	assert.Nil(t, parseClientIPThroughXFFMiddleware(r),
		"unparseable IP strings must yield nil so callers don't bind garbage into the HMAC payload")
}

func TestParseClientIP_ReturnsNilOnEmptyRequest(t *testing.T) {
	t.Parallel()

	r := httptest.NewRequest(http.MethodGet, "/mfa", nil)
	r.RemoteAddr = ""

	assert.Nil(t, parseClientIP(r),
		"no header + empty RemoteAddr must produce a nil IP")
}

func parseClientIPThroughXFFMiddleware(req *http.Request) net.IP {
	var ip net.IP
	router := chi.NewRouter()
	router.Use(chimiddleware.ClientIPFromXFF())
	router.Get("/mfa", func(w http.ResponseWriter, r *http.Request) {
		ip = parseClientIP(r)
		w.WriteHeader(http.StatusNoContent)
	})
	router.ServeHTTP(httptest.NewRecorder(), req)
	return ip
}
