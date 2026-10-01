package workerlease

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestMain gives every test its own tenant (#2419). The lease table is
// database-global, so each test also works on a disposable clone.
func TestMain(m *testing.M) {
	testpkg.PerTestTenants()
	testpkg.Run(m)
}
