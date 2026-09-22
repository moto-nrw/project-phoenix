package testdb

import (
	"encoding/hex"
	"hash/fnv"
)

// identifierHash returns the lower-case hex digest (32 characters) the
// package derives PostgreSQL identifiers and cache keys from. Names, template
// suffixes and fingerprints only need to be deterministic and collision
// resistant among a handful of values; they are never a security boundary,
// so the non-cryptographic 128-bit FNV-1a is enough and keeps test support
// off the crypto stack.
func identifierHash(value string) string {
	h := fnv.New128a()
	_, _ = h.Write([]byte(value))
	return hex.EncodeToString(h.Sum(nil))
}
