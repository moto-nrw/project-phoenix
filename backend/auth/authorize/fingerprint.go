package authorize

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
)

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
