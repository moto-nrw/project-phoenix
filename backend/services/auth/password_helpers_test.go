package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidatePasswordStrength tests password validation rules
func TestValidatePasswordStrength(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{
			name:     "valid password with all requirements",
			password: "Valid123!Pass",
			wantErr:  false,
		},
		{
			name:     "valid password minimum length",
			password: "Aa1!aaaa",
			wantErr:  false,
		},
		{
			name:     "valid password with special characters",
			password: "Complex!Pass123$",
			wantErr:  false,
		},
		{
			name:     "too short - 7 characters",
			password: "Ab1!abc",
			wantErr:  true,
		},
		{
			name:     "too short - empty",
			password: "",
			wantErr:  true,
		},
		{
			name:     "missing uppercase letter",
			password: "lowercase123!",
			wantErr:  true,
		},
		{
			name:     "missing lowercase letter",
			password: "UPPERCASE123!",
			wantErr:  true,
		},
		{
			name:     "missing digit",
			password: "NoDigits!Here",
			wantErr:  true,
		},
		{
			name:     "missing special character",
			password: "NoSpecial123",
			wantErr:  true,
		},
		{
			name:     "only lowercase",
			password: "alllowercase",
			wantErr:  true,
		},
		{
			name:     "only uppercase",
			password: "ALLUPPERCASE",
			wantErr:  true,
		},
		{
			name:     "only digits",
			password: "12345678",
			wantErr:  true,
		},
		{
			name:     "only special characters",
			password: "!@#$%^&*()",
			wantErr:  true,
		},
		{
			name:     "valid with spaces (space is special char)",
			password: "Pass Word 123",
			wantErr:  false,
		},
		{
			name:     "valid with various special chars",
			password: "P@ssw0rd!#$%",
			wantErr:  false,
		},
		{
			name:     "edge case - exactly 8 chars valid",
			password: "Abcd123!",
			wantErr:  false,
		},
		{
			name:     "long valid password",
			password: "ThisIsAVeryLongPasswordWith123!SpecialCharacters",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePasswordStrength(tt.password)
			if tt.wantErr {
				assert.ErrorIs(t, err, ErrPasswordTooWeak, "expected ErrPasswordTooWeak")
			} else {
				assert.NoError(t, err, "expected no error for valid password")
			}
		})
	}
}

// TestHashPassword tests that password hashing works and produces different hashes
func TestHashPassword(t *testing.T) {
	t.Run("produces a hash", func(t *testing.T) {
		password := "ValidPassword123!"
		hash, err := HashPassword(password)

		require.NoError(t, err)
		assert.NotEmpty(t, hash)
		assert.NotEqual(t, password, hash, "hash should not equal plaintext")
	})

	t.Run("produces different hashes for same password", func(t *testing.T) {
		password := "SamePassword123!"
		hash1, err1 := HashPassword(password)
		hash2, err2 := HashPassword(password)

		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.NotEqual(t, hash1, hash2, "hashes should be different due to salt")
	})

	t.Run("handles empty password", func(t *testing.T) {
		password := ""
		hash, err := HashPassword(password)

		// Should still produce a hash (validation is separate)
		require.NoError(t, err)
		assert.NotEmpty(t, hash)
	})

	t.Run("hash is Argon2id format", func(t *testing.T) {
		password := "TestPassword123!"
		hash, err := HashPassword(password)

		require.NoError(t, err)
		// Argon2id hashes start with $argon2id$
		assert.Contains(t, hash, "$argon2id$", "hash should be in Argon2id format")
	})
}
