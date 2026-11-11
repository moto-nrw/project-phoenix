package compose

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/adapters/tenantexport"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestMain gives every top-level test its own disposable database clone and
// tenant. Three of four tests here need a clone anyway: the demo run and the
// tenant export work on database-wide state. As explicit clones they shared
// the one slot of a package on the package database, so the package ran them
// one after another and its runtime was the sum of all clone lifecycles;
// under machine load that sum reached the package timeout (#6452).
//
// The exports skip the sync of each artifact: on a test directory on disk
// those syncs took most of the runtime (#6452). The tests of the return
// command (backend/cmd) keep them.
func TestMain(m *testing.M) {
	tenantexport.SkipFileSyncForTests()
	testpkg.PerTestDatabases()
	testpkg.PerTestTenants()
	testpkg.Run(m)
}
