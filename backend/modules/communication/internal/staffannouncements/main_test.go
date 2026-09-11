package announcement_test

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestMain gives every DB-backed test in this binary its own tenant (#2419).
// The internal mock-based tests in this package are unaffected.
func TestMain(m *testing.M) {
	testpkg.PerTestTenants()
	testpkg.Run(m)
}
