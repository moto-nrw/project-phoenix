package auth

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The login/MFA/reset flows under test hash passwords constantly; cheap
// Argon2id params keep that off the test suite's critical path.
//
// PerTestTenants gives every test in this binary its own tenant (#2419), so
// parallel tests in the shared package clone cannot see or overwrite each
// other's rows.
func TestMain(m *testing.M) {
	testpkg.UseCheapArgon2Params()
	testpkg.PerTestTenants()
	testpkg.Run(m)
}
