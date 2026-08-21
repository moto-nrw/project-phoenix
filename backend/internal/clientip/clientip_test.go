package clientip

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/assert"
)

func TestGetClientIPString_ChiXFFSingleHop(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.10")
	req.RemoteAddr = "172.18.0.4:54321"

	ip := getClientIPThroughMiddleware(req)

	assert.Equal(t, "203.0.113.10", ip)
}

func TestGetClientIPString_ChiXFFUsesRightmostUntrustedHop(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.10, 198.51.100.4, 192.0.2.1")
	req.RemoteAddr = "172.18.0.4:54321"

	ip := getClientIPThroughMiddleware(req)

	assert.Equal(t, "192.0.2.1", ip)
}

func TestGetClientIPString_MalformedXFFFallsBackToRemoteAddr(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.10, not-an-ip")
	req.RemoteAddr = "172.18.0.4:54321"

	ip := getClientIPThroughMiddleware(req)

	assert.Equal(t, "172.18.0.4", ip)
}

func TestGetClientIPString_RemoteAddrWithPort(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.5:54321"

	assert.Equal(t, "203.0.113.5", GetClientIPString(req))
}

func TestGetClientIPString_RemoteAddrWithoutPortReturnedRaw(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "no-port-here"

	assert.Equal(t, "no-port-here", GetClientIPString(req))
}

func TestGetClientIPString_IPv6BracketsHandled(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "[2001:db8::1]:54321"

	assert.Equal(t, "2001:db8::1", GetClientIPString(req))
}

func TestParseClientIP(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.10")
	req.RemoteAddr = "172.18.0.4:54321"

	var ip string
	router := chi.NewRouter()
	router.Use(chimiddleware.ClientIPFromXFF())
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		parsed := ParseClientIP(r)
		if parsed != nil {
			ip = parsed.String()
		}
		w.WriteHeader(http.StatusNoContent)
	})

	router.ServeHTTP(httptest.NewRecorder(), req)

	assert.Equal(t, "203.0.113.10", ip)
}

func TestParseIPString(t *testing.T) {
	t.Parallel()

	assert.Nil(t, ParseIPString(""))
	assert.Nil(t, ParseIPString("not-an-ip"))
	assert.Equal(t, "203.0.113.10", ParseIPString("203.0.113.10").String())
}

func getClientIPThroughMiddleware(req *http.Request) string {
	var ip string
	router := chi.NewRouter()
	router.Use(chimiddleware.ClientIPFromXFF())
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		ip = GetClientIPString(r)
		w.WriteHeader(http.StatusNoContent)
	})
	router.ServeHTTP(httptest.NewRecorder(), req)
	return ip
}
