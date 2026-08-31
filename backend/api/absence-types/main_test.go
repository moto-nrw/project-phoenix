package absencetypes

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func init() {
	testutil.SeedTestJWTConfig()
}

func TestMain(m *testing.M) {
	testpkg.PerTestTenants()
	testpkg.Run(m)
}
