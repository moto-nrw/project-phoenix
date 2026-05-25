package parent

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// getClientIP extracts the parent's real IP for login-attempt rate
// limiting + audit logging. Honors X-Forwarded-For (first hop) and
// X-Real-IP, with RemoteAddr as the final fallback. Differs from
// api/enrollment.remoteIPFromRequest only by also honoring X-Real-IP
// (used by older proxies).

func TestGetClientIP_XForwardedForFirstHop(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	r.Header.Set("X-Forwarded-For", "203.0.113.5, 198.51.100.4, 192.0.2.1")
	r.RemoteAddr = "10.0.0.1:54321"
	assert.Equal(t, "203.0.113.5", getClientIP(r),
		"first XFF hop wins — the rest is the proxy chain")
}

func TestGetClientIP_XForwardedForTrimsWhitespace(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	r.Header.Set("X-Forwarded-For", "  203.0.113.5  , 198.51.100.4")
	assert.Equal(t, "203.0.113.5", getClientIP(r))
}

func TestGetClientIP_XForwardedForBlankFirstHopFallsThrough(t *testing.T) {
	// A degenerate XFF where the first comma-separated value is empty
	// shouldn't return an empty string — fall through to X-Real-IP.
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	r.Header.Set("X-Forwarded-For", ", 198.51.100.4")
	r.Header.Set("X-Real-IP", "203.0.113.5")
	assert.Equal(t, "203.0.113.5", getClientIP(r),
		"blank XFF first hop must fall through, not return empty")
}

func TestGetClientIP_XRealIPSecondPriority(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	r.Header.Set("X-Real-IP", "203.0.113.5")
	r.RemoteAddr = "10.0.0.1:54321"
	assert.Equal(t, "203.0.113.5", getClientIP(r))
}

func TestGetClientIP_XForwardedForBeatsXRealIP(t *testing.T) {
	// When both headers are present, XFF wins — it's the modern
	// standard and our reverse proxy sets it.
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	r.Header.Set("X-Forwarded-For", "203.0.113.5")
	r.Header.Set("X-Real-IP", "198.51.100.4")
	assert.Equal(t, "203.0.113.5", getClientIP(r))
}

func TestGetClientIP_XRealIPTrimsWhitespace(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	r.Header.Set("X-Real-IP", "  203.0.113.5  ")
	assert.Equal(t, "203.0.113.5", getClientIP(r))
}

func TestGetClientIP_FallsBackToRemoteAddrSansPort(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	r.RemoteAddr = "203.0.113.5:54321"
	assert.Equal(t, "203.0.113.5", getClientIP(r))
}

func TestGetClientIP_RemoteAddrWithoutPortReturnedRaw(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	r.RemoteAddr = "no-port-here"
	assert.Equal(t, "no-port-here", getClientIP(r))
}

func TestGetClientIP_IPv6BracketsHandled(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	r.RemoteAddr = "[2001:db8::1]:54321"
	assert.Equal(t, "2001:db8::1", getClientIP(r))
}

func TestGetClientIP_NoHeadersNoAddrReturnsEmpty(t *testing.T) {
	// httptest sets a default RemoteAddr; explicitly blank it to
	// verify the empty-string return path.
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	r.RemoteAddr = ""
	assert.Equal(t, "", getClientIP(r))
}
