package schoolcalendar_test

import (
	"testing"

	testpkg "github.com/moto-ogs/project-phoenix/test"
)

func TestMain(m *testing.M) { testpkg.PerTestTenants(); testpkg.Run(m) }
