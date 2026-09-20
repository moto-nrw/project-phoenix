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

// assertDevOnlyTarget is the guard of the seed and simulate commands: the
// environment must be on the dev-only allow-list and the URL must be local.
// The internal host "server" is local in a deployed compose project too, so
// the URL alone cannot keep staging and production out.
func assertDevOnlyTarget(rawURL, appEnv string) error {
	if !isDevOnlyEnv(appEnv) {
		return fmt.Errorf("refusing to run with APP_ENV=%q — seed/simulate commands are dev-only (allowed: development, test, local, demo, or unset)", appEnv)
	}
	return assertNonProductionURL(rawURL)
}

// isDevOnlyEnv is the allow-list for the dev-only tools. Unset APP_ENV is the
// local-dev default. demo is the public demo environment (ADR 0027), which
// holds synthetic data only; staging, production, and any unknown value stay
// locked out.
func isDevOnlyEnv(appEnv string) bool {
	switch strings.ToLower(strings.TrimSpace(appEnv)) {
	case "", "local", "development", "dev", "test", "demo":
		return true
	default:
		return false
	}
}

func isLocalHostname(hostname string) bool {
	if hostname == "localhost" {
		return true
	}
	if ip := net.ParseIP(hostname); ip != nil && ip.IsLoopback() {
		return true
	}
	return localDockerHostnames[hostname]
}

var localDockerHostnames = map[string]bool{
	"server":                  true,
	"host.docker.internal":    true,
	"gateway.docker.internal": true,
}
