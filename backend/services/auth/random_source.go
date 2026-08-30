package auth

import (
	"crypto/rand"
	"io"
)

// SecureRandomSource supplies command roots with the same cryptographic
// entropy source used by authentication secrets.
func SecureRandomSource() io.Reader { return rand.Reader }
