package cmd

import (
	"fmt"
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
	// Loopback addresses
	if hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" {
		return true
	}

	// Bare hostnames without dots are Docker service names (e.g. "server", "postgres")
	if !strings.Contains(hostname, ".") {
		return true
	}

	// Common local suffixes
	if strings.HasSuffix(hostname, ".local") || strings.HasSuffix(hostname, ".internal") {
		return true
	}

	return false
}
