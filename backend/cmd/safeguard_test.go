package cmd

import (
	"testing"
)

func TestDemoTargetAllowsOnlyDemoOrLocalInternalServer(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		environment, url string
		allowed          bool
	}{
		{"demo", "http://server:8080", true},
		{"local", "http://localhost:8080", true},
		{"development", "http://127.0.0.1:8080", true},
		{"demo", "http://localhost:8080", false},
		{"production", "http://server:8080", false},
		{"staging", "http://localhost:8080", false},
		{"demo", "https://demo.moto-app.de", false},
		{"local", "ftp://localhost", false},
	} {
		err := validateDemoTarget(tc.url, tc.environment)
		if (err == nil) != tc.allowed {
			t.Errorf("environment=%s url=%s: allowed=%v, error=%v", tc.environment, tc.url, tc.allowed, err)
		}
	}
}

func TestAssertNonProductionURL(t *testing.T) {
	t.Parallel()
	allowed := []string{
		"http://localhost:8080",
		"http://127.0.0.1:8080",
		"http://127.0.0.2:8080",
		"http://[::1]:8080",
		"http://server:8080",               // Docker service name
		"http://host.docker.internal:8080", // Docker desktop loopback bridge
	}

	blocked := []string{
		"https://api.moto-app.de",
		"https://demo.moto-app.de",
		"https://moto-app.de",
		"https://app.moto.nrw",
		"https://moto.nrw",
		"https://example.com",
		"https://my-server.cloud.provider.com:8080",
		"https://staging.some-domain.de",
		"http://staging:8080",
		"https://api.prod.internal",
		"http://postgres:5432",
		"http://myhost.local:8080",
	}

	for _, u := range allowed {
		if err := assertNonProductionURL(u); err != nil {
			t.Errorf("expected %q to be allowed, got: %v", u, err)
		}
	}

	for _, u := range blocked {
		if err := assertNonProductionURL(u); err == nil {
			t.Errorf("expected %q to be blocked, but it was allowed", u)
		}
	}
}

// The internal host "server" is local in every compose project, so APP_ENV
// decides: the public demo environment passes, staging and production do not.
func TestAssertDevOnlyTarget(t *testing.T) {
	t.Parallel()

	for _, env := range []string{"", "local", "development", "dev", "test", "demo", "DEMO", "  demo  "} {
		if err := assertDevOnlyTarget("http://server:8080", env); err != nil {
			t.Errorf("expected APP_ENV=%q to be allowed, got: %v", env, err)
		}
	}
	for _, env := range []string{"staging", "production", "prod", "preview", "demo-staging"} {
		if err := assertDevOnlyTarget("http://server:8080", env); err == nil {
			t.Errorf("expected APP_ENV=%q to be blocked, but it was allowed", env)
		}
	}
	if err := assertDevOnlyTarget("https://demo.moto-app.de", "demo"); err == nil {
		t.Error("expected the public demo host to be blocked even with APP_ENV=demo")
	}
}
