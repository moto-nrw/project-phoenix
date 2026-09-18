package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The MFA e-mail codes are hashed with the project-wide Argon2id helper, so
// tuning its parameters reaches them too (#3331).
func TestShortCodeHasherRoundTrip(t *testing.T) {
	t.Parallel()

	hasher := shortCodeHasher{}
	const code = "421337"

	hash, err := hasher.HashShortCode(code)
	require.NoError(t, err)
	assert.NotEqual(t, code, hash, "the hash must not contain the plaintext")
	assert.Contains(t, hash, "$argon2id$", "expected the Argon2id prefix")
	assert.Len(t, code, 6, "the mailed code is six digits")

	ok, err := hasher.VerifyShortCode(code, hash)
	require.NoError(t, err)
	assert.True(t, ok, "a freshly hashed code must verify")

	bad, err := hasher.VerifyShortCode("000000", hash)
	require.NoError(t, err)
	assert.False(t, bad, "a wrong code must not verify")
}
