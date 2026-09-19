package behavior_test

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The behaviour suites in this package drive the Identity & Access flows the
// retained services/auth and services/platform used to hold (#3364): login,
// the second factor, both portals' passkey ceremonies, the invitations, the
// guardian relative access, the staff preview and the operator flows. They
// run against a real test database and the composed service root, so what
// they pin is what production runs.
//
// The flows hash passwords constantly; cheap Argon2id params keep that off
// the test suite's critical path. PerTestTenants gives every test in this
// binary its own tenant (#2419), so parallel tests in the shared package
// clone cannot see or overwrite each other's rows.
func TestMain(m *testing.M) {
	testpkg.UseCheapArgon2Params()
	testpkg.PerTestTenants()
	testpkg.Run(m)
}
