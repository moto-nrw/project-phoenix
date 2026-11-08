package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRecoveryProofIsHashedAndConstantTimeMatched(t *testing.T) {
	t.Parallel()
	proof := NewRecoveryProof("independent-recovery-secret")
	hash := proof.Hash()
	assert.Len(t, hash, 32)
	assert.NotEqual(t, []byte("independent-recovery-secret"), hash)
	assert.True(t, proof.Matches(hash))
	assert.False(t, NewRecoveryProof("different-token").Matches(hash))
	assert.False(t, NewRecoveryProof("").Matches(hash), "a missing proof recovers nothing")
	assert.Nil(t, NewRecoveryProof("").Hash())
	assert.False(t, proof.Matches(nil), "a hand-off without a stored digest is not recoverable")
}
