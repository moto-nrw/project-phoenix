package users

import (
	"os"
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/userpass"
)

// TestMain makes Argon2id hashing cheap for every test in this binary.
// Production DefaultParams (64MiB memory, 3 iterations) cost 55-120ms per
// hash; guardian account creation and staff PIN flows under test hash on
// most paths. The override only applies to callers that pass nil params;
// each encoded hash self-describes its params, so verification is
// unaffected.
func TestMain(m *testing.M) {
	userpass.DefaultOverride = &userpass.PasswordParams{
		Memory:      1024, // KiB
		Iterations:  1,
		Parallelism: 1,
		SaltLength:  16,
		KeyLength:   32,
	}
	os.Exit(m.Run())
}
