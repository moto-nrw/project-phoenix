package application

import (
	"crypto/rand"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Status tokens of an intake composed with the CSPRNG the root binds.

func statusTokenIntake() *Intake {
	return NewIntake(IntakeDependencies{Random: func(b []byte) error {
		_, err := rand.Read(b)
		return err
	}})
}

func TestNewStatusToken_DecodesToThirtyTwoBytes(t *testing.T) {
	t.Parallel()

	token, err := statusTokenIntake().newStatusToken()
	require.NoError(t, err)
	raw, err := base64.RawURLEncoding.DecodeString(token)
	require.NoError(t, err, "token must be RawURLEncoding base64")
	assert.Len(t, raw, 32, "32 random bytes per CSPRNG guarantee")
}

func TestNewStatusToken_UnpaddedRawURLEncoding(t *testing.T) {
	t.Parallel()

	token, err := statusTokenIntake().newStatusToken()
	require.NoError(t, err)
	// RawURLEncoding strips the trailing '=' padding. 32 bytes →
	// 43 base64 chars, no padding.
	assert.Len(t, token, 43)
	for _, r := range token {
		// URL-safe alphabet only: a-z A-Z 0-9 - _.
		ok := (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '-' || r == '_'
		assert.True(t, ok, "char %q must be in the RawURL alphabet", r)
	}
}

func TestNewStatusToken_TokensAreDistinct(t *testing.T) {
	t.Parallel()

	svc := statusTokenIntake()
	seen := make(map[string]struct{}, 32)
	for i := 0; i < 32; i++ {
		token, err := svc.newStatusToken()
		require.NoError(t, err)
		_, dup := seen[token]
		require.False(t, dup, "CSPRNG must produce distinct tokens")
		seen[token] = struct{}{}
	}
}

// An intake composed without an entropy source refuses to mint a token
// instead of handing out a predictable one.
func TestNewStatusToken_RequiresEntropySource(t *testing.T) {
	t.Parallel()

	token, err := NewIntake(IntakeDependencies{}).newStatusToken()
	require.Error(t, err)
	assert.Empty(t, token)
}
