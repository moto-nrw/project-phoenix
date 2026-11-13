package students

import (
	"testing"

	"github.com/moto-ogs/project-phoenix/api/testutil"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

func TestMain(m *testing.M) { testpkg.PerTestTenants(); testutil.SeedTestJWTConfig(); testpkg.Run(m) }
