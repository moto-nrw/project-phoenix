package application

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestMain gives every test in this binary its own tenant (#2419); the
// lifecycle's after-commit tests open the test database.
func TestMain(m *testing.M) {
	testpkg.PerTestTenants()
	testpkg.Run(m)
}
