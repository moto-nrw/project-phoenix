package compose_test

import (
	"testing"

	// The registry definitions the wizard resolves (presence mode, group
	// mode, care plan toggle).
	_ "github.com/moto-nrw/project-phoenix/services/config/defaults"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestMain gives every test in this binary its own tenant (#2419).
func TestMain(m *testing.M) {
	testpkg.PerTestTenants()
	testpkg.Run(m)
}
