package users

import (
	"os"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Guardian account creation and staff PIN flows hash on most paths; cheap
// Argon2id params keep that off the test suite's critical path.
func TestMain(m *testing.M) {
	testpkg.UseCheapArgon2Params()
	os.Exit(m.Run())
}
