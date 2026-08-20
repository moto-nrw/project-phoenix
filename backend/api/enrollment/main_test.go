package enrollment

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestMain gives every test in this binary its own tenant (#2419), so parallel
// tests in the shared package clone cannot see or overwrite each other's rows.
func TestMain(m *testing.M) {
	testutil.SeedTestJWTConfig()
	testpkg.PerTestTenants()
	testpkg.Run(m)
}
