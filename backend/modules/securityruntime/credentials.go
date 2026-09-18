package securityruntime

import "github.com/moto-nrw/project-phoenix/auth/authorize"

// The credential primitives the compositions hand to Identity & Access and
// the operator flows. Security Runtime owns the password policy and the
// hashing parameters; its consumers state which credential they are hashing,
// never how (#3364).

// HashPassword hashes a plain-text password using the default parameters.
func HashPassword(password string) (string, error) {
	return authorize.HashPassword(password)
}

// VerifyPassword checks a plain-text password against its Argon2id hash. It
// is the credential check the Identity & Access login flows are composed
// with, so password hashing stays in one place.
func VerifyPassword(password, hash string) (bool, error) {
	return authorize.VerifyPassword(password, hash)
}

// PasswordTooWeak reports a password that does not meet the minimum
// complexity requirements. The caller reports the refusal in its own error
// contract.
func PasswordTooWeak(password string) bool {
	return authorize.PasswordTooWeak(password)
}

// FillRandom fills b from the cryptographic entropy source authentication
// secrets are drawn from. The command roots wrap it in the reader their
// generators expect; Security Runtime hands out the bytes, not the stream.
func FillRandom(b []byte) error {
	return authorize.FillRandom(b)
}
