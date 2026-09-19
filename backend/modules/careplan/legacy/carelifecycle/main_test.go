package carelifecycle

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The care-exit, companion and document suites create staff accounts, so the
// cheap Argon2id params keep hashing off the critical path.
//
// PerTestTenants gives every test in this binary its own tenant (#2419), so
// parallel tests in the shared package clone cannot see or overwrite each
// other's rows — which the lock-order and audit-history assertions depend on.
func TestMain(m *testing.M) {
	testpkg.UseCheapArgon2Params()
	testpkg.PerTestTenants()
	testpkg.Run(m)
}
