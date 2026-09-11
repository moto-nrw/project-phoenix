// Package tokens supplies feed secrets and stable calendar fingerprints.
package tokens

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// New returns a 256-bit URL-safe capability token. Only its digest is stored.
func New() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate calendar feed token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func Digest(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}
