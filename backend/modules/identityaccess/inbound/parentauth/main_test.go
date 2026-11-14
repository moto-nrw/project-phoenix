package parentauth_test

import (
	"testing"

	"github.com/moto-ogs/project-phoenix/api/testutil"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

func TestMain(m *testing.M) {
	testutil.SeedTestJWTConfig()
	testpkg.PerTestTenants()
	testpkg.Run(m)
}
