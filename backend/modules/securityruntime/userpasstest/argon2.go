// Package userpasstest holds the Security Runtime's test-only Argon2id
// profile, so test binaries hash throwaway credentials cheaply without
// reaching into the hashing package themselves.
package userpasstest

import "github.com/moto-nrw/project-phoenix/modules/securityruntime/internal/domain/userpass"

// cheapParams is the test-only Argon2id profile. Production DefaultParams
// (64MiB memory, 3 iterations) cost 55-120ms per hash; this profile is
// sub-2ms. The params are encoded into each hash, so verification
// self-describes and stays correct.
var cheapParams = &userpass.PasswordParams{
	Memory:      1024, // KiB
	Iterations:  1,
	Parallelism: 1,
	SaltLength:  16,
	KeyLength:   32,
}

// UseCheapParams makes Argon2id hashing cheap for the calling test binary.
// Call it from a package's TestMain when its tests hash passwords or PINs
// through code paths that pass nil params to userpass.HashPassword.
func UseCheapParams() {
	userpass.DefaultOverride = cheapParams
}

// Hash returns the Argon2id hash of password with the cheap profile.
func Hash(password string) (string, error) {
	return userpass.HashPassword(password, cheapParams)
}
