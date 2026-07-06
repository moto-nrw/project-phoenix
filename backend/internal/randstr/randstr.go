// Package randstr generates cryptographically secure random strings. It
// replaces the per-package crypto/rand generator copies consolidated in the
// 2026-07 code-reduction audit (B7).
package randstr

import (
	"crypto/rand"
	"encoding/hex"
	"math/big"
)

// Alphanumeric is the mixed-case alphanumeric alphabet.
const Alphanumeric = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// LowerAlphanumeric is the lowercase alphanumeric alphabet.
const LowerAlphanumeric = "abcdefghijklmnopqrstuvwxyz0123456789"

// String returns a uniformly random string of length n over charset.
func String(n int, charset string) (string, error) {
	b := make([]byte, n)
	max := big.NewInt(int64(len(charset)))
	for i := range b {
		idx, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b[i] = charset[idx.Int64()]
	}
	return string(b), nil
}

// APIKey returns a device API key: 32 random bytes hex-encoded with the
// "dev_" prefix.
func APIKey() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "dev_" + hex.EncodeToString(buf), nil
}
