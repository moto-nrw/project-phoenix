package test

import (
	"io"

	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

// ContentFingerprint verifies SHA-256 artifact identities in behavior tests
// without giving those suites a general cryptographic implementation import.
func ContentFingerprint(content []byte) string { return securityruntime.Fingerprint(content) }

func ReaderContentFingerprint(reader io.Reader) (string, int64, error) {
	return securityruntime.ReaderFingerprint(reader)
}
