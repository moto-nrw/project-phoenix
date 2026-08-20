// Deliberately NOT parallel (whole package): the cmd tests drive cobra
// commands and initConfig, which read and write the viper singleton and
// os.Stdout. Nothing here is isolated from the next test, so no test in this
// package calls t.Parallel() — said once here instead of above each of the
// 134 tests (#2419).
package cmd

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestMain gives every test in this binary its own tenant (#2419), so parallel
// tests in the shared package clone cannot see or overwrite each other's rows.
func TestMain(m *testing.M) {
	testpkg.PerTestTenants()
	m.Run()
}
