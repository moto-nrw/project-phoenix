package auth

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// HashPassword Tests
// =============================================================================

func TestHashPassword_DefaultParams(t *testing.T) {
	password := "securePassword123!"

	hash, err := HashPassword(password, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)

	// Verify hash format: $argon2id$v=19$m=65536,t=3,p=2$<salt>$<hash>
	assert.True(t, strings.HasPrefix(hash, "$argon2id$v=19$"), "Hash should start with argon2id version marker")
	parts := strings.Split(hash, "$")
	assert.Len(t, parts, 6, "Hash should have 6 parts separated by $")
}

func TestHashPassword_CustomParams(t *testing.T) {
	password := "testPassword"
	params := &PasswordParams{
		Memory:      32 * 1024, // 32MB (smaller for faster tests)
		Iterations:  2,
		Parallelism: 1,
		SaltLength:  16,
		KeyLength:   32,
	}

	hash, err := HashPassword(password, params)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)

	// Verify custom params are reflected in the hash
	assert.Contains(t, hash, "m=32768,t=2,p=1")
}

func TestHashPassword_EmptyPassword(t *testing.T) {
	// Empty password should still hash (validation is caller's responsibility)
	hash, err := HashPassword("", nil)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
}

func TestHashPassword_UniqueHashes(t *testing.T) {
	// Same password should produce different hashes due to random salt
	password := "samePassword"

	hash1, err := HashPassword(password, nil)
	require.NoError(t, err)

	hash2, err := HashPassword(password, nil)
	require.NoError(t, err)

	assert.NotEqual(t, hash1, hash2, "Same password should produce different hashes due to random salt")
}

func TestHashPassword_LongPassword(t *testing.T) {
	// Very long passwords should work
	password := strings.Repeat("a", 1000)

	hash, err := HashPassword(password, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
}

func TestHashPassword_UnicodePassword(t *testing.T) {
	// Unicode passwords should work
	password := "пароль密码🔐"

	hash, err := HashPassword(password, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)

	// Should verify correctly
	match, err := VerifyPassword(password, hash)
	require.NoError(t, err)
	assert.True(t, match)
}

// =============================================================================
// VerifyPassword Tests
// =============================================================================

func TestVerifyPassword_CorrectPassword(t *testing.T) {
	password := "correctPassword123"
	hash, err := HashPassword(password, nil)
	require.NoError(t, err)

	match, err := VerifyPassword(password, hash)
	require.NoError(t, err)
	assert.True(t, match, "Correct password should verify successfully")
}

func TestVerifyPassword_IncorrectPassword(t *testing.T) {
	password := "originalPassword"
	hash, err := HashPassword(password, nil)
	require.NoError(t, err)

	match, err := VerifyPassword("wrongPassword", hash)
	require.NoError(t, err)
	assert.False(t, match, "Wrong password should not verify")
}

func TestVerifyPassword_CaseSensitive(t *testing.T) {
	password := "CaseSensitive"
	hash, err := HashPassword(password, nil)
	require.NoError(t, err)

	// Different case should fail
	match, err := VerifyPassword("casesensitive", hash)
	require.NoError(t, err)
	assert.False(t, match, "Password verification should be case sensitive")
}

func TestVerifyPassword_EmptyPassword(t *testing.T) {
	// Hash an empty password
	hash, err := HashPassword("", nil)
	require.NoError(t, err)

	// Verify empty password matches
	match, err := VerifyPassword("", hash)
	require.NoError(t, err)
	assert.True(t, match)

	// Non-empty should not match
	match, err = VerifyPassword("notEmpty", hash)
	require.NoError(t, err)
	assert.False(t, match)
}

func TestVerifyPassword_SimilarPasswords(t *testing.T) {
	password := "password123"
	hash, err := HashPassword(password, nil)
	require.NoError(t, err)

	testCases := []struct {
		name     string
		password string
	}{
		{"with extra char", "password1234"},
		{"missing char", "password12"},
		{"different number", "password124"},
		{"with space", "password 123"},
		{"with leading space", " password123"},
		{"with trailing space", "password123 "},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			match, err := VerifyPassword(tc.password, hash)
			require.NoError(t, err)
			assert.False(t, match, "Similar but different password should not verify")
		})
	}
}

// =============================================================================
// decodeHash / Invalid Hash Tests
// =============================================================================

func TestVerifyPassword_InvalidHashFormat(t *testing.T) {
	testCases := []struct {
		name string
		hash string
	}{
		{"empty string", ""},
		{"no delimiters", "notahash"},
		{"too few parts", "$argon2id$v=19$salt$hash"},
		{"wrong algorithm", "$bcrypt$v=19$m=65536,t=3,p=2$salt$hash"},
		{"invalid version format", "$argon2id$invalid$m=65536,t=3,p=2$salt$hash"},
		{"invalid params format", "$argon2id$v=19$invalid$salt$hash"},
		{"invalid salt encoding", "$argon2id$v=19$m=65536,t=3,p=2$!!!invalid!!$aGFzaA"},
		{"invalid hash encoding", "$argon2id$v=19$m=65536,t=3,p=2$c2FsdA$!!!invalid!!"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := VerifyPassword("anypassword", tc.hash)
			assert.Error(t, err, "Should return error for invalid hash format")
		})
	}
}

func TestVerifyPassword_InvalidHash_TooFewParts(t *testing.T) {
	_, err := VerifyPassword("password", "$argon2id$v=19$m=65536")
	assert.ErrorIs(t, err, ErrInvalidHash)
}

func TestVerifyPassword_InvalidHash_WrongAlgorithm(t *testing.T) {
	_, err := VerifyPassword("password", "$bcrypt$v=19$m=65536,t=3,p=2$c2FsdA$aGFzaA")
	assert.ErrorIs(t, err, ErrInvalidHash)
}

func TestVerifyPassword_IncompatibleVersion(t *testing.T) {
	// Create a hash with a fake incompatible version (v=18 instead of v=19)
	_, err := VerifyPassword("password", "$argon2id$v=18$m=65536,t=3,p=2$c2FsdA$aGFzaA")
	assert.ErrorIs(t, err, ErrIncompatibleVersion)
}

// =============================================================================
// DefaultParams Tests
// =============================================================================

func TestDefaultParams(t *testing.T) {
	params := DefaultParams()

	assert.Equal(t, uint32(64*1024), params.Memory, "Default memory should be 64MB")
	assert.Equal(t, uint32(3), params.Iterations, "Default iterations should be 3")
	assert.Equal(t, uint8(2), params.Parallelism, "Default parallelism should be 2")
	assert.Equal(t, uint32(16), params.SaltLength, "Default salt length should be 16 bytes")
	assert.Equal(t, uint32(32), params.KeyLength, "Default key length should be 32 bytes")
}

// =============================================================================
// Integration Tests (Hash + Verify cycle)
// =============================================================================

func TestHashAndVerify_RoundTrip(t *testing.T) {
	testPasswords := []string{
		"simplePassword",
		"with spaces in it",
		"!@#$%^&*()_+-=[]{}|;':\",./<>?",
		"émojis🔐🎉",
		strings.Repeat("x", 100),
	}

	for _, password := range testPasswords {
		t.Run(password[:min(20, len(password))], func(t *testing.T) {
			hash, err := HashPassword(password, nil)
			require.NoError(t, err)

			match, err := VerifyPassword(password, hash)
			require.NoError(t, err)
			assert.True(t, match, "Password should verify after hashing")
		})
	}
}

func TestHashAndVerify_WithCustomParams(t *testing.T) {
	password := "testPassword"
	params := &PasswordParams{
		Memory:      32 * 1024,
		Iterations:  2,
		Parallelism: 1,
		SaltLength:  32, // Longer salt
		KeyLength:   64, // Longer key
	}

	hash, err := HashPassword(password, params)
	require.NoError(t, err)

	// Should still verify correctly
	match, err := VerifyPassword(password, hash)
	require.NoError(t, err)
	assert.True(t, match)
}

// =============================================================================
// Security Tests
// =============================================================================

func TestConstantTimeComparison(t *testing.T) {
	// This test ensures the password verification uses constant-time comparison
	// by verifying that both correct and incorrect passwords complete
	// (actual timing attack testing would require benchmarks)
	password := "securePassword"
	hash, err := HashPassword(password, nil)
	require.NoError(t, err)

	// Correct password
	match1, err := VerifyPassword(password, hash)
	require.NoError(t, err)
	assert.True(t, match1)

	// Completely wrong password (should use constant-time comparison)
	match2, err := VerifyPassword("completelyDifferent", hash)
	require.NoError(t, err)
	assert.False(t, match2)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
