package application

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Status tokens of an intake. The root binds the CSPRNG as the entropy
// source; module-internal tests may not import crypto, so these use a source
// that fills every draw with fresh bytes and pin that each token consumes one
// full 32-byte draw.

func statusTokenIntake() *Intake {
	var draw byte
	return NewIntake(IntakeDependencies{Random: func(b []byte) error {
		draw++
		for i := range b {
			b[i] = draw ^ byte(i*37)
		}
		return nil
	}})
}

func TestNewStatusToken_DecodesToThirtyTwoBytes(t *testing.T) {
	t.Parallel()

	token, err := statusTokenIntake().newStatusToken()
	require.NoError(t, err)
	raw, err := base64.RawURLEncoding.DecodeString(token)
	require.NoError(t, err, "token must be RawURLEncoding base64")
	assert.Len(t, raw, 32, "32 random bytes per token")
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
		require.False(t, dup, "every token must come from a fresh draw")
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
