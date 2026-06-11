package auth

import (
	"os"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The login/MFA/reset flows under test hash passwords constantly; cheap
// Argon2id params keep that off the test suite's critical path.
func TestMain(m *testing.M) {
	testpkg.UseCheapArgon2Params()
	os.Exit(m.Run())
}
