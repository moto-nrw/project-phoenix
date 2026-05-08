package cmd

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// assertNonProductionURL fails fast if the URL points to a production or public host.
// Seed and simulate commands must only run against local development servers.
func assertNonProductionURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}

	hostname := strings.ToLower(parsed.Hostname())
	if hostname == "" {
		return fmt.Errorf("URL %q has no hostname", rawURL)
	}

	if isLocalHostname(hostname) {
		return nil
	}

	return fmt.Errorf("refusing to run against %q — seed/simulate commands are for local development only", rawURL)
}

func isLocalHostname(hostname string) bool {
	if hostname == "localhost" {
		return true
	}

	// Allow any loopback IP, not just 127.0.0.1 / ::1.
	if ip := net.ParseIP(hostname); ip != nil && ip.IsLoopback() {
		return true
	}

	// Only allow explicit local Docker hostnames used in development.
	if localDockerHostnames[hostname] {
		return true
	}

	return false
}

// Keep in sync with the canonical E2E harness:
// `go run . e2e run` / `go run . e2e up` boot the isolated backend and the
// Playwright harness reads the resulting Go-owned E2E state file.
var localDockerHostnames = map[string]bool{
	"server":                  true,
	"backend":                 true,
	"host.docker.internal":    true,
	"gateway.docker.internal": true,
}
