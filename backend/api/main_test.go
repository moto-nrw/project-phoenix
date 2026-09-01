package api

import (
	"os"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestMain gives every test in this binary its own tenant (#2419), so parallel
// tests in the shared package clone cannot see or overwrite each other's rows.
func TestMain(m *testing.M) {
	if err := os.Setenv("METRICS_BEARER_TOKEN", "route-golden-test-token"); err != nil {
		panic(err)
	}
	testpkg.PerTestTenants()
	testpkg.Run(m)
}
