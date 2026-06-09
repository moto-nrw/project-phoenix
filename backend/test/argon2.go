package test

import "github.com/moto-nrw/project-phoenix/auth/userpass"

// cheapArgon2Params is the test-only Argon2id profile. Production
// DefaultParams (64MiB memory, 3 iterations) cost 55-120ms per hash; this
// profile is sub-2ms. The params are encoded into each hash, so
// verification self-describes and stays correct.
var cheapArgon2Params = &userpass.PasswordParams{
	Memory:      1024, // KiB
	Iterations:  1,
	Parallelism: 1,
	SaltLength:  16,
	KeyLength:   32,
}

// UseCheapArgon2Params makes Argon2id hashing cheap for the calling test
// binary. Call it from a package's TestMain when its tests hash passwords
// or PINs through code paths that pass nil params to userpass.HashPassword.
func UseCheapArgon2Params() {
	userpass.DefaultOverride = cheapArgon2Params
}
