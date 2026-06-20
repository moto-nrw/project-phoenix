package seedtoken

import (
	"net"
	"net/http"
	"strings"
)

const Header = "X-Phoenix-Seed-Token"

var allowedLocalHostnames = map[string]bool{
	"localhost":               true,
	"server":                  true,
	"host.docker.internal":    true,
	"gateway.docker.internal": true,
}

// ShouldExposeInvitationToken gates seed-only raw invitation token exposure.
// API handlers call this only to support local demo seeding; deployed
// environments must never receive tokens in normal invitation responses.
func ShouldExposeInvitationToken(r *http.Request, appEnv string) bool {
	if r == nil {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(r.Header.Get(Header)), "true") {
		return false
	}
	if !isAllowedEnvironment(appEnv) {
		return false
	}
	return isLocalHost(requestHostname(r))
}

func isAllowedEnvironment(appEnv string) bool {
	switch strings.ToLower(strings.TrimSpace(appEnv)) {
	case "development", "dev", "local", "test":
		return true
	default:
		return false
	}
}

func requestHostname(r *http.Request) string {
	host := ""
	if r != nil {
		host = strings.TrimSpace(r.Host)
		if host == "" && r.URL != nil {
			host = strings.TrimSpace(r.URL.Host)
		}
	}
	if host == "" {
		return ""
	}

	if hostname, _, err := net.SplitHostPort(host); err == nil {
		return strings.ToLower(strings.Trim(hostname, "[]"))
	}
	return strings.ToLower(strings.Trim(host, "[]"))
}

func isLocalHost(hostname string) bool {
	if allowedLocalHostnames[hostname] {
		return true
	}
	ip := net.ParseIP(hostname)
	return ip != nil && ip.IsLoopback()
}
