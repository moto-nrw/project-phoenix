package authorize

import (
	"crypto/rand"
	"regexp"

	"github.com/moto-nrw/project-phoenix/auth/userpass"
)

// The credential decisions Security Runtime owns: what a stored password
// looks like, whether a plain one may be accepted, and where generated
// secrets get their entropy. Consumers reach them through the Security
// Runtime facade and state which credential they are hashing, never how
// (#3364).

// HashPassword hashes a plain-text password using the default parameters.
// Passing nil (instead of DefaultParams()) lets test binaries swap in cheap
// Argon2id params via userpass.DefaultOverride; outside tests the two are
// identical.
func HashPassword(password string) (string, error) {
	return userpass.HashPassword(password, nil)
}

// VerifyPassword checks a plain-text password against its Argon2id hash.
func VerifyPassword(password, hash string) (bool, error) {
	return userpass.VerifyPassword(password, hash)
}

// PasswordTooWeak reports a password that does not meet the minimum
// complexity requirements: at least eight characters including an upper-case
// letter, a lower-case letter, a digit and a non-alphanumeric character. The
// caller reports the refusal in its own error contract.
func PasswordTooWeak(password string) bool {
	if len(password) < 8 {
		return true
	}
	for _, class := range passwordClasses {
		if !class.MatchString(password) {
			return true
		}
	}
	return false
}

var passwordClasses = []*regexp.Regexp{
	regexp.MustCompile(`[A-Z]`),
	regexp.MustCompile(`[a-z]`),
	regexp.MustCompile(`[0-9]`),
	regexp.MustCompile(`[^a-zA-Z0-9]`),
}

// FillRandom fills b from the cryptographic entropy source authentication
// secrets are drawn from.
func FillRandom(b []byte) error {
	_, err := rand.Read(b)
	return err
}
