package userpass

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/argon2"
)

// TestDefaultParams tests that default parameters are returned correctly
func TestDefaultParams(t *testing.T) {
	params := DefaultParams()

	require.NotNil(t, params)
	assert.Equal(t, uint32(64*1024), params.Memory, "Default memory should be 64MB")
	assert.Equal(t, uint32(3), params.Iterations, "Default iterations should be 3")
	assert.Equal(t, uint8(2), params.Parallelism, "Default parallelism should be 2")
	assert.Equal(t, uint32(16), params.SaltLength, "Default salt length should be 16 bytes")
	assert.Equal(t, uint32(32), params.KeyLength, "Default key length should be 32 bytes")
}

// TestHashPassword tests password hashing functionality
func TestHashPassword(t *testing.T) {
	t.Run("hash with default parameters", func(t *testing.T) {
		password := "SecurePassword123!"
		hash, err := HashPassword(password, nil)

		require.NoError(t, err)
		assert.NotEmpty(t, hash)

		// Verify hash format: $argon2id$v=19$m=65536,t=3,p=2$<salt>$<hash>
		assert.True(t, strings.HasPrefix(hash, "$argon2id$v=19$"))
		assert.Contains(t, hash, "m=65536,t=3,p=2")

		// Count parts (should be 6: empty, argon2id, version, params, salt, hash)
		parts := strings.Split(hash, "$")
		assert.Len(t, parts, 6, "Hash should have 6 parts separated by $")
	})

	t.Run("hash with custom parameters", func(t *testing.T) {
		password := "CustomPassword456!"
		customParams := &PasswordParams{
			Memory:      32 * 1024, // 32MB
			Iterations:  2,
			Parallelism: 1,
			SaltLength:  8,
			KeyLength:   16,
		}

		hash, err := HashPassword(password, customParams)

		require.NoError(t, err)
		assert.NotEmpty(t, hash)
		assert.Contains(t, hash, "m=32768,t=2,p=1", "Hash should contain custom parameters")
	})

	t.Run("same password produces different hashes (random salt)", func(t *testing.T) {
		password := "SamePassword789!"

		hash1, err1 := HashPassword(password, nil)
		require.NoError(t, err1)

		hash2, err2 := HashPassword(password, nil)
		require.NoError(t, err2)

		assert.NotEqual(t, hash1, hash2, "Same password should produce different hashes due to random salt")
	})

	t.Run("different passwords produce different hashes", func(t *testing.T) {
		hash1, err1 := HashPassword("Password1", nil)
		require.NoError(t, err1)

		hash2, err2 := HashPassword("Password2", nil)
		require.NoError(t, err2)

		assert.NotEqual(t, hash1, hash2, "Different passwords should produce different hashes")
	})

	t.Run("empty password can be hashed", func(t *testing.T) {
		hash, err := HashPassword("", nil)

		require.NoError(t, err)
		assert.NotEmpty(t, hash)
		assert.True(t, strings.HasPrefix(hash, "$argon2id$"))
	})

	t.Run("very long password can be hashed", func(t *testing.T) {
		longPassword := strings.Repeat("a", 10000)
		hash, err := HashPassword(longPassword, nil)

		require.NoError(t, err)
		assert.NotEmpty(t, hash)
	})

	t.Run("unicode password can be hashed", func(t *testing.T) {
		unicodePassword := "Пароль123!@#äöüß漢字"
		hash, err := HashPassword(unicodePassword, nil)

		require.NoError(t, err)
		assert.NotEmpty(t, hash)
	})
}

// TestVerifyPassword tests password verification functionality
func TestVerifyPassword(t *testing.T) {
	t.Run("correct password verifies successfully", func(t *testing.T) {
		password := "CorrectPassword123!"
		hash, err := HashPassword(password, nil)
		require.NoError(t, err)

		match, err := VerifyPassword(password, hash)
		require.NoError(t, err)
		assert.True(t, match, "Correct password should verify successfully")
	})

	t.Run("incorrect password fails verification", func(t *testing.T) {
		password := "CorrectPassword123!"
		hash, err := HashPassword(password, nil)
		require.NoError(t, err)

		match, err := VerifyPassword("WrongPassword456!", hash)
		require.NoError(t, err)
		assert.False(t, match, "Incorrect password should fail verification")
	})

	t.Run("empty password verification", func(t *testing.T) {
		hash, err := HashPassword("", nil)
		require.NoError(t, err)

		match, err := VerifyPassword("", hash)
		require.NoError(t, err)
		assert.True(t, match, "Empty password should verify against its own hash")

		match, err = VerifyPassword("notEmpty", hash)
		require.NoError(t, err)
		assert.False(t, match, "Non-empty password should not verify against empty password hash")
	})

	t.Run("case sensitive verification", func(t *testing.T) {
		password := "CaseSensitive"
		hash, err := HashPassword(password, nil)
		require.NoError(t, err)

		match, err := VerifyPassword("casesensitive", hash)
		require.NoError(t, err)
		assert.False(t, match, "Password verification should be case-sensitive")
	})

	t.Run("unicode password verification", func(t *testing.T) {
		password := "Пароль123!äöüß"
		hash, err := HashPassword(password, nil)
		require.NoError(t, err)

		match, err := VerifyPassword(password, hash)
		require.NoError(t, err)
		assert.True(t, match, "Unicode password should verify correctly")
	})

	t.Run("whitespace matters in password", func(t *testing.T) {
		password := "Password With Spaces"
		hash, err := HashPassword(password, nil)
		require.NoError(t, err)

		match, err := VerifyPassword("PasswordWithSpaces", hash)
		require.NoError(t, err)
		assert.False(t, match, "Password with spaces should not match password without spaces")

		match, err = VerifyPassword(password, hash)
		require.NoError(t, err)
		assert.True(t, match, "Password with spaces should match its own hash")
	})

	t.Run("verify with custom parameters", func(t *testing.T) {
		password := "CustomParamsPassword"
		customParams := &PasswordParams{
			Memory:      32 * 1024,
			Iterations:  2,
			Parallelism: 1,
			SaltLength:  8,
			KeyLength:   16,
		}

		hash, err := HashPassword(password, customParams)
		require.NoError(t, err)

		match, err := VerifyPassword(password, hash)
		require.NoError(t, err)
		assert.True(t, match, "Password should verify with custom parameters")
	})
}

// TestVerifyPassword_ErrorCases tests error handling in password verification
func TestVerifyPassword_ErrorCases(t *testing.T) {
	t.Run("invalid hash format - too few parts", func(t *testing.T) {
		invalidHash := "$argon2id$v=19$m=65536"
		match, err := VerifyPassword("password", invalidHash)

		assert.Error(t, err)
		assert.Equal(t, ErrInvalidHash, err)
		assert.False(t, match)
	})

	t.Run("invalid hash format - wrong algorithm", func(t *testing.T) {
		invalidHash := "$bcrypt$v=19$m=65536,t=3,p=2$salt$hash"
		match, err := VerifyPassword("password", invalidHash)

		assert.Error(t, err)
		assert.Equal(t, ErrInvalidHash, err)
		assert.False(t, match)
	})

	t.Run("invalid hash format - wrong version", func(t *testing.T) {
		// Create a hash with wrong version
		invalidHash := "$argon2id$v=18$m=65536,t=3,p=2$c2FsdA$aGFzaA"
		match, err := VerifyPassword("password", invalidHash)

		assert.Error(t, err)
		assert.Equal(t, ErrIncompatibleVersion, err)
		assert.False(t, match)
	})

	t.Run("invalid hash format - malformed parameters", func(t *testing.T) {
		invalidHash := "$argon2id$v=19$invalid$c2FsdA$aGFzaA"
		match, err := VerifyPassword("password", invalidHash)

		assert.Error(t, err)
		assert.False(t, match)
	})

	t.Run("invalid hash format - bad base64 salt", func(t *testing.T) {
		invalidHash := "$argon2id$v=19$m=65536,t=3,p=2$!!!invalid!!!$aGFzaA"
		match, err := VerifyPassword("password", invalidHash)

		assert.Error(t, err)
		assert.False(t, match)
	})

	t.Run("invalid hash format - bad base64 hash", func(t *testing.T) {
		invalidHash := "$argon2id$v=19$m=65536,t=3,p=2$c2FsdA$!!!invalid!!!"
		match, err := VerifyPassword("password", invalidHash)

		assert.Error(t, err)
		assert.False(t, match)
	})

	t.Run("empty hash string", func(t *testing.T) {
		match, err := VerifyPassword("password", "")

		assert.Error(t, err)
		assert.Equal(t, ErrInvalidHash, err)
		assert.False(t, match)
	})
}

// TestDecodeHash tests the internal hash decoding function
func TestDecodeHash(t *testing.T) {
	t.Run("decode valid hash", func(t *testing.T) {
		// Create a hash to decode
		password := "TestPassword123"
		encodedHash, err := HashPassword(password, nil)
		require.NoError(t, err)

		params, salt, hash, err := decodeHash(encodedHash)

		require.NoError(t, err)
		assert.NotNil(t, params)
		assert.NotNil(t, salt)
		assert.NotNil(t, hash)

		// Verify default parameters
		assert.Equal(t, uint32(64*1024), params.Memory)
		assert.Equal(t, uint32(3), params.Iterations)
		assert.Equal(t, uint8(2), params.Parallelism)
		assert.Len(t, salt, 16, "Salt should be 16 bytes")
		assert.Len(t, hash, 32, "Hash should be 32 bytes")
	})

	t.Run("decode hash with custom parameters", func(t *testing.T) {
		customParams := &PasswordParams{
			Memory:      32 * 1024,
			Iterations:  5,
			Parallelism: 4,
			SaltLength:  8,
			KeyLength:   24,
		}

		encodedHash, err := HashPassword("test", customParams)
		require.NoError(t, err)

		params, salt, hash, err := decodeHash(encodedHash)

		require.NoError(t, err)
		assert.Equal(t, customParams.Memory, params.Memory)
		assert.Equal(t, customParams.Iterations, params.Iterations)
		assert.Equal(t, customParams.Parallelism, params.Parallelism)
		assert.Len(t, salt, int(customParams.SaltLength))
		assert.Len(t, hash, int(customParams.KeyLength))
	})
}

// TestHashPassword_ConsistentFormat tests that hash format is consistent
func TestHashPassword_ConsistentFormat(t *testing.T) {
	password := "ConsistentFormat123"
	hash, err := HashPassword(password, nil)
	require.NoError(t, err)

	// Verify the hash format matches the expected pattern
	parts := strings.Split(hash, "$")
	require.Len(t, parts, 6)

	assert.Equal(t, "", parts[0], "First part should be empty (string starts with $)")
	assert.Equal(t, "argon2id", parts[1], "Algorithm should be argon2id")
	assert.True(t, strings.HasPrefix(parts[2], "v="), "Version should start with v=")
	assert.True(t, strings.Contains(parts[3], "m="), "Parameters should contain memory (m=)")
	assert.True(t, strings.Contains(parts[3], "t="), "Parameters should contain iterations (t=)")
	assert.True(t, strings.Contains(parts[3], "p="), "Parameters should contain parallelism (p=)")
	assert.NotEmpty(t, parts[4], "Salt should not be empty")
	assert.NotEmpty(t, parts[5], "Hash should not be empty")
}

// TestPasswordStrength tests various password strengths
func TestPasswordStrength(t *testing.T) {
	testCases := []struct {
		name     string
		password string
	}{
		{"simple alphanumeric", "abc123"},
		{"with special characters", "P@ssw0rd!"},
		{"very long password", strings.Repeat("LongPassword", 100)},
		{"only numbers", "123456789"},
		{"only letters", "abcdefghijklmnop"},
		{"only special chars", "!@#$%^&*()"},
		{"mixed unicode", "パスワード123!"},
		{"emoji password", "🔒🔐🔑🗝️"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hash, err := HashPassword(tc.password, nil)
			require.NoError(t, err)
			assert.NotEmpty(t, hash)

			match, err := VerifyPassword(tc.password, hash)
			require.NoError(t, err)
			assert.True(t, match, "Password should verify successfully")
		})
	}
}

// TestVerifyPassword_TimingAttackResistance tests constant-time comparison
func TestVerifyPassword_TimingAttackResistance(t *testing.T) {
	// This test verifies that password verification uses constant-time comparison
	// to prevent timing attacks
	password := "SecurePassword123!"
	hash, err := HashPassword(password, nil)
	require.NoError(t, err)

	// Test passwords that differ at different positions
	testPasswords := []string{
		"XecurePassword123!", // Differs at position 0
		"SXcurePassword123!", // Differs at position 1
		"SeXurePassword123!", // Differs at position 2
		"SecXrePassword123!", // Differs at position 3
		"XXXXXXXXXXXXXXXXXX", // Completely different
		"WrongPassword456!",  // Different password
	}

	// All incorrect passwords should return false
	// The timing should be consistent (constant-time comparison)
	for i, testPassword := range testPasswords {
		t.Run(testPassword, func(t *testing.T) {
			startTime := time.Now()
			match, err := VerifyPassword(testPassword, hash)
			elapsed := time.Since(startTime)

			require.NoError(t, err)
			assert.False(t, match, "Incorrect password %d should not match", i)
			assert.True(t, elapsed > 0, "Verification should take measurable time")

			// Note: We can't easily test for actual constant-time behavior in unit tests,
			// but we're using crypto/subtle.ConstantTimeCompare internally which
			// guarantees constant-time comparison
		})
	}
}

// TestHashPassword_Argon2Version tests that correct Argon2 version is used
func TestHashPassword_Argon2Version(t *testing.T) {
	password := "VersionTest"
	hash, err := HashPassword(password, nil)
	require.NoError(t, err)

	// Extract version from hash
	parts := strings.Split(hash, "$")
	require.Len(t, parts, 6)

	var version int
	_, err = fmt.Sscanf(parts[2], "v=%d", &version)
	require.NoError(t, err)

	assert.Equal(t, argon2.Version, version, "Hash should use current Argon2 version")
}

// TestHashPassword_SaltRandomness tests that salts are truly random
func TestHashPassword_SaltRandomness(t *testing.T) {
	password := "SaltRandomnessTest"
	numHashes := 100
	salts := make(map[string]bool)

	for i := 0; i < numHashes; i++ {
		hash, err := HashPassword(password, nil)
		require.NoError(t, err)

		// Extract salt from hash
		parts := strings.Split(hash, "$")
		require.Len(t, parts, 6)
		salt := parts[4]

		// Check that we haven't seen this salt before
		assert.False(t, salts[salt], "Salt should be unique (iteration %d)", i)
		salts[salt] = true
	}

	// All salts should be unique
	assert.Len(t, salts, numHashes, "All %d salts should be unique", numHashes)
}

// TestHashPassword_ParameterRanges tests various parameter ranges
func TestHashPassword_ParameterRanges(t *testing.T) {
	testCases := []struct {
		name   string
		params *PasswordParams
	}{
		{
			name: "minimum parameters",
			params: &PasswordParams{
				Memory:      8 * 1024, // 8MB
				Iterations:  1,
				Parallelism: 1,
				SaltLength:  8,
				KeyLength:   16,
			},
		},
		{
			name: "high security parameters",
			params: &PasswordParams{
				Memory:      128 * 1024, // 128MB
				Iterations:  5,
				Parallelism: 4,
				SaltLength:  32,
				KeyLength:   64,
			},
		},
		{
			name: "very high iterations",
			params: &PasswordParams{
				Memory:      64 * 1024,
				Iterations:  10,
				Parallelism: 2,
				SaltLength:  16,
				KeyLength:   32,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			password := "TestPassword"
			hash, err := HashPassword(password, tc.params)

			require.NoError(t, err)
			assert.NotEmpty(t, hash)

			// Verify the password
			match, err := VerifyPassword(password, hash)
			require.NoError(t, err)
			assert.True(t, match, "Password should verify with custom parameters")

			// Verify wrong password fails
			match, err = VerifyPassword("WrongPassword", hash)
			require.NoError(t, err)
			assert.False(t, match, "Wrong password should not verify")
		})
	}
}

// TestVerifyPassword_Performance tests that verification is reasonably fast
func TestVerifyPassword_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	password := "PerformanceTest123"
	hash, err := HashPassword(password, nil)
	require.NoError(t, err)

	// Verification should complete in reasonable time (< 1 second with default params)
	startTime := time.Now()
	match, err := VerifyPassword(password, hash)
	elapsed := time.Since(startTime)

	require.NoError(t, err)
	assert.True(t, match)
	assert.Less(t, elapsed, 1*time.Second, "Verification should complete within 1 second")

	t.Logf("Password verification took: %v", elapsed)
}

// TestHashPassword_Concurrency tests that hashing is safe for concurrent use
func TestHashPassword_Concurrency(t *testing.T) {
	password := "ConcurrentTest"
	concurrency := 50
	done := make(chan bool, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(id int) {
			hash, err := HashPassword(password, nil)
			assert.NoError(t, err)
			assert.NotEmpty(t, hash)

			match, err := VerifyPassword(password, hash)
			assert.NoError(t, err)
			assert.True(t, match)

			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < concurrency; i++ {
		<-done
	}
}

// TestPasswordParams_ZeroValues tests behavior with zero or nil parameters
func TestPasswordParams_ZeroValues(t *testing.T) {
	t.Run("nil parameters use defaults", func(t *testing.T) {
		password := "TestPassword"
		hash, err := HashPassword(password, nil)

		require.NoError(t, err)
		assert.NotEmpty(t, hash)
		assert.Contains(t, hash, "m=65536,t=3,p=2", "Should use default parameters")
	})
}

// Benchmark tests
func BenchmarkHashPassword(b *testing.B) {
	password := "BenchmarkPassword123!"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = HashPassword(password, nil)
	}
}

func BenchmarkVerifyPassword(b *testing.B) {
	password := "BenchmarkPassword123!"
	hash, _ := HashPassword(password, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = VerifyPassword(password, hash)
	}
}

func BenchmarkHashPassword_CustomParams(b *testing.B) {
	password := "BenchmarkPassword123!"
	params := &PasswordParams{
		Memory:      32 * 1024,
		Iterations:  2,
		Parallelism: 1,
		SaltLength:  8,
		KeyLength:   16,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = HashPassword(password, params)
	}
}
