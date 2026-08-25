package billing_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func init() {
	// The Router() tests drive the production JWT middleware chain, which
	// refuses to verify without a configured secret.
	testutil.SeedTestJWTConfig()
}

// TestMain gives every test in this binary its own tenant (#2419).
func TestMain(m *testing.M) {
	testpkg.PerTestTenants()
	testpkg.Run(m)
}
