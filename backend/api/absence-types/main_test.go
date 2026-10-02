package absencetypes_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil/routetest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func init() {
	routetest.SeedTestJWTConfig()
}

func TestMain(m *testing.M) {
	testpkg.PerTestTenants()
	testpkg.Run(m)
}
