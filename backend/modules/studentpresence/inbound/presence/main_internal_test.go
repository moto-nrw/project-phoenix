package presence

import (
	"testing"

	"github.com/moto-ogs/project-phoenix/api/testutil"

	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// Configure JWT signing once before parallel tests and isolate their tenants.
func TestMain(m *testing.M) {
	testpkg.PerTestTenants()
	testutil.SeedTestJWTConfig()
	testpkg.Run(m)
}
