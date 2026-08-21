package auth

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateEmailCode(t *testing.T) {
	t.Parallel()

	for i := 0; i < 50; i++ {
		code, err := GenerateEmailCode()
		require.NoError(t, err)
		assert.Len(t, code, MFAEmailCodeLength, "code must be 6 digits")
		for _, r := range code {
			assert.True(t, r >= '0' && r <= '9', "every char must be a decimal digit")
		}
	}
}

func TestHashShortCode_RoundTripEmailCode(t *testing.T) {
	t.Parallel()

	code, err := GenerateEmailCode()
	require.NoError(t, err)

	hash, err := HashShortCode(code)
	require.NoError(t, err)
	assert.NotEqual(t, code, hash, "hash must not contain plaintext")
	assert.Contains(t, hash, "$argon2id$", "expected Argon2id prefix")

	ok, err := VerifyShortCode(code, hash)
	require.NoError(t, err)
	assert.True(t, ok, "freshly hashed code must verify")

	bad, err := VerifyShortCode("000000", hash)
	require.NoError(t, err)
	assert.False(t, bad, "wrong code must not verify")
}

func TestTrustedDeviceToken_GenerateHashSignVerify(t *testing.T) {
	t.Parallel()

	raw, err := GenerateTrustedDeviceToken()
	require.NoError(t, err)
	assert.NotEmpty(t, raw)

	// SHA-256 hex is exactly 64 characters.
	dbHash := HashTrustedDeviceToken(raw)
	assert.Len(t, dbHash, 64)

	secret := DeriveMFASecret("a-very-long-test-secret-with-enough-entropy")
	require.NotEmpty(t, secret)

	signed := SignTrustedDeviceToken(raw, secret)
	assert.Contains(t, signed, ".", "signed token must contain a separator")

	got, ok := VerifyTrustedDeviceToken(signed, secret)
	require.True(t, ok, "expected signature verification to succeed")
	assert.Equal(t, raw, got)
}

func TestTrustedDeviceToken_RejectsTamperedSignature(t *testing.T) {
	t.Parallel()

	raw, err := GenerateTrustedDeviceToken()
	require.NoError(t, err)

	secret := DeriveMFASecret("test-jwt-secret-that-is-long-enough")
	signed := SignTrustedDeviceToken(raw, secret)

	// Flip a byte in the signature half.
	tamperIdx := strings.LastIndex(signed, ".") + 1
	require.Greater(t, tamperIdx, 0)
	tampered := signed[:tamperIdx] + flipByte(signed[tamperIdx:tamperIdx+1]) + signed[tamperIdx+1:]
	require.NotEqual(t, signed, tampered)

	_, ok := VerifyTrustedDeviceToken(tampered, secret)
	assert.False(t, ok, "tampered signature must be rejected")
}

func TestTrustedDeviceToken_RejectsWrongSecret(t *testing.T) {
	t.Parallel()

	raw, err := GenerateTrustedDeviceToken()
	require.NoError(t, err)

	signed := SignTrustedDeviceToken(raw, DeriveMFASecret("secret-A"))
	_, ok := VerifyTrustedDeviceToken(signed, DeriveMFASecret("secret-B"))
	assert.False(t, ok)
}

func TestTrustedDeviceToken_RejectsMalformed(t *testing.T) {
	t.Parallel()

	secret := DeriveMFASecret("any-secret")
	for _, in := range []string{"", "no-dot-at-all", ".", "raw.", ".sig"} {
		_, ok := VerifyTrustedDeviceToken(in, secret)
		assert.False(t, ok, "input %q must not verify", in)
	}
}

func TestDeriveMFASecret_DeterministicAndDistinct(t *testing.T) {
	t.Parallel()

	a := DeriveMFASecret("jwt-secret")
	b := DeriveMFASecret("jwt-secret")
	assert.Equal(t, a, b, "same input must produce same secret")

	c := DeriveMFASecret("different-secret")
	assert.NotEqual(t, a, c, "different inputs must produce different secrets")

	assert.Nil(t, DeriveMFASecret(""), "empty input must return nil")
}

func flipByte(s string) string {
	if len(s) == 0 {
		return s
	}
	b := []byte(s)
	if b[0] == 'A' {
		b[0] = 'B'
	} else {
		b[0] = 'A'
	}
	return string(b)
}
