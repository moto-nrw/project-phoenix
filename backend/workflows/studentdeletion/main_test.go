package studentdeletion_test

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestMain(m *testing.M) {
	testpkg.UseCheapArgon2Params()
	testpkg.PerTestTenants()
	testpkg.Run(m)
}
