package compose

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestMain gives every top-level test its own disposable database clone and
// tenant. Three of four tests here need a clone anyway: the demo run and the
// tenant export work on database-wide state. As explicit clones they shared
// the one slot of a package on the package database, so the package ran them
// one after another and its runtime was the sum of all clone lifecycles;
// under machine load that sum reached the package timeout (#6452).
func TestMain(m *testing.M) {
	testpkg.PerTestDatabases()
	testpkg.PerTestTenants()
	testpkg.Run(m)
}
