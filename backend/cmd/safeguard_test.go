package cmd

import (
	"testing"
)

func TestAssertNonProductionURL(t *testing.T) {
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
