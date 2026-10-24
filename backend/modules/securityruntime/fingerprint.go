// Package securityruntime exposes security primitives without leaking their
// implementation into application workflows.
package securityruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"io"

	"github.com/moto-nrw/project-phoenix/modules/securityruntime/internal/application/authorize"
)

// Fingerprint returns a stable SHA-256 content identity. It is not a password
// hash or a secret authenticator and must not be used for either purpose.
func Fingerprint(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// ReaderFingerprint streams r into the same SHA-256 content identity and
// returns it with the number of bytes read.
func ReaderFingerprint(r io.Reader) (string, int64, error) {
	return authorize.ReaderFingerprint(r)
}
