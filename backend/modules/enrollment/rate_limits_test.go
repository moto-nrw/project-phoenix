package enrollment

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- SubmissionRateLimit -------------------------------------------------

func TestSubmissionRateLimitKeyType_StableValues(t *testing.T) {
	t.Parallel()

	// The repository's IncrementAttempts upserts on (tenant, key_type,
	// key_value). A rename of these constants would split the bucket
	// into pre-rename and post-rename rows, silently doubling effective
	// throughput.
	assert.Equal(t, "ip", SubmissionRateLimitKeyTypeIP)
	assert.Equal(t, "email", SubmissionRateLimitKeyTypeEmail)
}
