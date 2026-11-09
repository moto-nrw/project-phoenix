package api

import (
	"os"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestMain gives every test in this binary its own tenant (#2419), so parallel
// tests in the shared package clone cannot see or overwrite each other's rows.
// emailWebhookTestSecret signs test bodies only.
const emailWebhookTestSecret = "whsec_router_test" //nolint:gosec // fake value for a test body

func TestMain(m *testing.M) {
	testpkg.UseCheapArgon2Params()
	if err := os.Setenv("METRICS_BEARER_TOKEN", "route-golden-test-token"); err != nil {
		panic(err)
	}
	// The golden test sends a signed delivery through the complete router.
	if err := os.Setenv("EMAIL_WEBHOOK_SECRETS", emailWebhookTestSecret); err != nil {
		panic(err)
	}
	testpkg.PerTestTenants()
	testpkg.Run(m)
}
