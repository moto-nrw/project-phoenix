package test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/securityruntime/userpasstest"
)

// UseCheapArgon2Params makes Argon2id hashing cheap for the calling test
// binary. Call it from a package's TestMain when its tests hash passwords
// or PINs through code paths that pass nil params to userpass.HashPassword.
func UseCheapArgon2Params() {
	userpasstest.UseCheapParams()
}

// HashTestPassword returns the Argon2id hash of a throwaway test credential
// (a password, a second-factor code) with the cheap test profile, so a
// behaviour suite can write a row the production verifier accepts without
// naming the hashing owner's packages.
func HashTestPassword(tb testing.TB, password string) string {
	tb.Helper()
	hash, err := hashPassword(password)
	if err != nil {
		tb.Fatalf("hash test password: %v", err)
	}
	return hash
}
