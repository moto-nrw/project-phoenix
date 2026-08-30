// Deliberately NOT parallel (whole package): the settings registry is a
// package-global map. These tests register, override and reset definitions in
// it (registerTestSetting and friends), so a test that reads a definition
// cannot run beside one that replaces it. Said once here instead of above
// each of the ~147 tests (#2419).
package config

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
