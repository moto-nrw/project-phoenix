package rotation

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRecoveryPolicyIsBounded(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 5*time.Minute, RecoveryGrace)
	assert.Positive(t, MaxRecoveryHops)
	assert.LessOrEqual(t, MaxRecoveryHops, 8)
}

func TestRecoveryProofIsHashedAndConstantTimeMatched(t *testing.T) {
	t.Parallel()

	ctx := WithRecoveryProof(context.Background(), "independent-recovery-secret")
	hash := RecoveryProofHash(ctx)
	assert.Len(t, hash, 32)
	assert.NotEqual(t, []byte("independent-recovery-secret"), hash)
	assert.True(t, MatchesRecoveryProof(ctx, hash))
	assert.False(t, MatchesRecoveryProof(WithRecoveryProof(context.Background(), "different-token"), hash))
	assert.False(t, MatchesRecoveryProof(context.Background(), hash))
}
