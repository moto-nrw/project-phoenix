// Deliberately NOT parallel (whole package): these tests apply and roll back
// migrations, i.e. they change the SCHEMA of the clone every test in the
// binary shares. Two of them at once would each see the other's half-applied
// state. The source-only registry collision check is the explicit exception.
// Said once here instead of above each of the ~88 tests (#2419).
package migrations

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestMain gives every test in this binary its own tenant (#2419), so parallel
// tests in the shared package clone cannot see or overwrite each other's rows.
func TestMain(m *testing.M) {
	testpkg.PerTestTenants()
	testpkg.Run(m)
}
