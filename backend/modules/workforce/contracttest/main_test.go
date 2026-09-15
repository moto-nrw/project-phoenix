// Package contracttest_test verifies the retained planning and substitution
// adapters through their public capabilities, without extending runtime wiring.
package contracttest_test

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestMain(m *testing.M) {
	testpkg.PerTestTenants()
	testpkg.Run(m)
}
