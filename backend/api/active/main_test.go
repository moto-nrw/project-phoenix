package active_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Configure JWT signing once before parallel tests and isolate their tenants.
func TestMain(m *testing.M) {
	testpkg.PerTestTenants()
	testutil.SeedTestJWTConfig()
	testpkg.Run(m)
}
