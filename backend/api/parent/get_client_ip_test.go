package parent

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/assert"
)

// getClientIP returns the router-selected client IP for login-attempt rate
// limiting + audit logging. XFF semantics come from chi's ClientIPFromXFF.

func TestGetClientIP_XForwardedForRightmostHop(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	r.Header.Set("X-Forwarded-For", "203.0.113.5, 198.51.100.4, 192.0.2.1")
	r.RemoteAddr = "10.0.0.1:54321"
	assert.Equal(t, "192.0.2.1", getClientIPThroughXFFMiddleware(r))
}

func TestGetClientIP_XForwardedForTrimsWhitespace(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	r.Header.Set("X-Forwarded-For", "  203.0.113.5  , 198.51.100.4")
	assert.Equal(t, "198.51.100.4", getClientIPThroughXFFMiddleware(r))
}

func TestGetClientIP_MalformedXForwardedForFallsBackToRemoteAddr(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	r.Header.Set("X-Forwarded-For", "203.0.113.5, not-an-ip")
	r.Header.Set("X-Real-IP", "198.51.100.4")
	r.RemoteAddr = "10.0.0.1:54321"
	assert.Equal(t, "10.0.0.1", getClientIPThroughXFFMiddleware(r))
}

func TestGetClientIP_IgnoresRawXRealIP(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	r.Header.Set("X-Real-IP", "203.0.113.5")
	r.RemoteAddr = "10.0.0.1:54321"
	assert.Equal(t, "10.0.0.1", getClientIP(r))
}

func TestGetClientIP_XForwardedForBeatsXRealIP(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	r.Header.Set("X-Forwarded-For", "203.0.113.5")
	r.Header.Set("X-Real-IP", "198.51.100.4")
	assert.Equal(t, "203.0.113.5", getClientIPThroughXFFMiddleware(r))
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

func getClientIPThroughXFFMiddleware(req *http.Request) string {
	var ip string
	router := chi.NewRouter()
	router.Use(chimiddleware.ClientIPFromXFF())
	router.Post("/x", func(w http.ResponseWriter, r *http.Request) {
		ip = getClientIP(r)
		w.WriteHeader(http.StatusNoContent)
	})
	router.ServeHTTP(httptest.NewRecorder(), req)
	return ip
}
