package clientip

import (
	"net"
	"net/http"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// GetClientIPString returns the request client IP selected by the router's
// client-IP middleware, falling back to RemoteAddr for tests and non-router
// call sites.
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

// ParseClientIP returns the selected request client IP as net.IP.
func ParseClientIP(r *http.Request) net.IP {
	return ParseIPString(GetClientIPString(r))
}

// ParseIPString parses a client IP string, preserving nil for empty or invalid
// inputs so callers do not persist malformed inet values.
func ParseIPString(ipAddress string) net.IP {
	if ipAddress == "" {
		return nil
	}
	return net.ParseIP(ipAddress)
}
