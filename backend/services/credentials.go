package services

import (
	"io"

	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

// The dev-only command roots (`seed`, `seed-parents`) need the same entropy
// source and the same password hash the authentication flows use, and they
// may not reach Security Runtime themselves. The composition root hands both
// through, so no command root carries its own copy of either (#3364).

// secureRandom streams the cryptographic entropy Security Runtime owns.
type secureRandom struct{}

func (secureRandom) Read(b []byte) (int, error) {
	if err := securityruntime.FillRandom(b); err != nil {
		return 0, err
	}
	return len(b), nil
}

// SecureRandomSource is the entropy source the seeder draws generated
// credentials from.
func SecureRandomSource() io.Reader { return secureRandom{} }

// HashPassword produces the Argon2id hash the account rows store.
func HashPassword(password string) (string, error) {
	return securityruntime.HashPassword(password)
}
