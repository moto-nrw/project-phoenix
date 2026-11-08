package rotation

import (
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
