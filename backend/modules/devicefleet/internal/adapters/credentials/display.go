package credentials

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

// DisplayTokens mints 32 random bytes as URL-safe base64 and persists only
// the SHA-256 hex digest, so a database leak exposes no usable dashboard URL.
type DisplayTokens struct{}

func (DisplayTokens) Mint() (string, string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", "", fmt.Errorf("failed to generate display token: %w", err)
	}
	raw := base64.RawURLEncoding.EncodeToString(buffer)
	return raw, DisplayTokens{}.Hash(raw), nil
}

func (DisplayTokens) Hash(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:])
}
