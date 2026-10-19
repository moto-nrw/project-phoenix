package owners

import (
	"io"

	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

// secureRandom streams the cryptographic entropy Security Runtime owns.
type secureRandom struct{}

func (secureRandom) Read(b []byte) (int, error) {
	if err := securityruntime.FillRandom(b); err != nil {
		return 0, err
	}
	return len(b), nil
}

// SecureRandomSource is the entropy source of the demo school orders.
func SecureRandomSource() io.Reader { return secureRandom{} }
