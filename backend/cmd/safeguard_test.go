package cmd

import (
	"testing"
)

func TestAssertNonProductionURL(t *testing.T) {
	allowed := []string{
		"http://localhost:8080",
		"http://127.0.0.1:8080",
		"http://[::1]:8080",
		"http://server:8080",       // Docker service name
		"http://postgres:5432",     // Docker service name
		"http://myhost.local:8080", // .local suffix
		"http://app.internal:8080", // .internal suffix
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
