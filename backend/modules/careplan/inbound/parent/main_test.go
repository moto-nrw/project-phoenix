package parent_test

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The JWT config is process-global viper state and identical for every test in
// this package. The testpkg import pins it to testpkg.TestJWTSecret in its own
// init, before any router or token-auth singleton exists, so no test here
// writes viper while others run in parallel (#2419).
func TestMain(m *testing.M) {
	testpkg.PerTestTenants()
	testpkg.Run(m)
}
