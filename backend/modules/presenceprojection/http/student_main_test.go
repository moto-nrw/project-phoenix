package presenceprojectionhttp_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestMain(m *testing.M) { testpkg.PerTestTenants(); testutil.SeedTestJWTConfig(); testpkg.Run(m) }
