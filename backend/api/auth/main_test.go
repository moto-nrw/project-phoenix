package auth

import (
	"os"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The login/reset handler tests hash passwords on most paths; cheap
// Argon2id params keep that off the test suite's critical path. Per-test
// tenants (#2419) keep parallel tests in the shared package clone from
// seeing or overwriting each other's rows.
func TestMain(m *testing.M) {
	testpkg.UseCheapArgon2Params()
	testpkg.PerTestTenants()
	os.Exit(m.Run())
}
