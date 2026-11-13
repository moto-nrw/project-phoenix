package absencetypes_test

import (
	"testing"

	"github.com/moto-ogs/project-phoenix/api/testutil/routetest"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

func init() {
	routetest.SeedTestJWTConfig()
}

func TestMain(m *testing.M) {
	testpkg.PerTestTenants()
	testpkg.Run(m)
}
