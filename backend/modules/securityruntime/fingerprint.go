// Package securityruntime exposes security primitives without leaking their
// implementation into application workflows.
package securityruntime

import (
	"crypto/sha256"
	"encoding/hex"
)

// Fingerprint returns a stable SHA-256 content identity. It is not a password
// hash or a secret authenticator and must not be used for either purpose.
func Fingerprint(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
