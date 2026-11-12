package migrations

import (
	"testing"

	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// TestMain gives every top-level test its own disposable database clone and
// tenant (#2419). Dropping the clone replaces row-by-row tenant teardown;
// migration tests never share a mutable schema or data with the next test.
func TestMain(m *testing.M) {
	testpkg.PerTestDatabases()
	testpkg.PerTestTenants()
	testpkg.Run(m)
}
