package common

import (
	"net"
	"net/http"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// GetClientIPString returns the client IP selected by the root router. Direct
// resource tests may attach Chi's client-IP middleware without the root
// RemoteAddr synchronizer, so the selected context value is checked first.
func GetClientIPString(r *http.Request) string {
	if r == nil {
		return ""
	}
	if ip := chimiddleware.GetClientIP(r.Context()); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func ParseClientIP(r *http.Request) net.IP {
	return net.ParseIP(GetClientIPString(r))
}
