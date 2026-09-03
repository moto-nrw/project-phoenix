package httpadapter

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestMain(m *testing.M) {
	testpkg.PerTestTenants()
	testpkg.Run(m)
}
