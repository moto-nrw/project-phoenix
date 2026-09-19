package jwt

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// NewOpaqueCapabilityToken creates a bearer secret suitable for an
// unauthenticated capability URL and returns only its non-reversible database
// fingerprint alongside it.
func NewOpaqueCapabilityToken() (string, string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", fmt.Errorf("create opaque capability token: %w", err)
	}
	raw := base64.RawURLEncoding.EncodeToString(bytes)
	return raw, OpaqueCapabilityFingerprint(raw), nil
}

// OpaqueCapabilityFingerprint returns the SHA-256 fingerprint persisted for a
// capability token. The raw bearer secret must never be stored or logged.
func OpaqueCapabilityFingerprint(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
