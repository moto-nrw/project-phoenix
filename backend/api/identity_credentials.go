package api

import (
	"io"

	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

// The demo access and the dev-only command roots (`demo`, `seed`) draw
// generated credentials from the cryptographic entropy Security Runtime owns
// (#3364).

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
