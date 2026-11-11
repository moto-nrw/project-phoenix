// Package securityruntime exposes security primitives without leaking their
// implementation into application workflows.
package securityruntime

import (
	"crypto/hmac"
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

// KeyedFingerprint returns the HMAC-SHA256 of content under key as hex: a
// pseudonym that only the key holder can derive from a guessable value such
// as an e-mail address (ADR 0196). It is not a password hash.
func KeyedFingerprint(key, content []byte) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(content)
	return hex.EncodeToString(mac.Sum(nil))
}

// ReaderFingerprint streams r into the same SHA-256 content identity and
// returns it with the number of bytes read.
func ReaderFingerprint(r io.Reader) (string, int64, error) {
	return authorize.ReaderFingerprint(r)
}
