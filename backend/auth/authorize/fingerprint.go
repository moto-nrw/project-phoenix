package authorize

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
)

// ContentFingerprint returns the SHA-256 (hex) of content: a stable content
// identity for proof records such as an Erklärung (#3430). It is neither a
// password hash nor a secret authenticator.
func ContentFingerprint(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// ReaderFingerprint streams r into SHA-256 and returns the hex digest and the
// number of bytes read.
func ReaderFingerprint(r io.Reader) (string, int64, error) {
	hash := sha256.New()
	size, err := io.Copy(hash, r)
	if err != nil {
		return "", size, err
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}
