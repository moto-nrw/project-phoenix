package enrollment

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---- SubmissionDedupLockKey ---------------------------------------------

func TestSubmissionDedupLockKey_Stable(t *testing.T) {
	t.Parallel()

	// Stability matters: the value is used as an advisory-lock key on
	// the lowercased guardian email so concurrent submissions for the
	// same parent serialize. Changing the algorithm would silently
	// allow duplicates through during a rolling deploy.
	a := SubmissionDedupLockKey("parent@example.com")
	b := SubmissionDedupLockKey("parent@example.com")
	assert.Equal(t, a, b)
}

func TestSubmissionDedupLockKey_DifferentInputsCollideRarely(t *testing.T) {
	t.Parallel()

	assert.NotEqual(t,
		SubmissionDedupLockKey("parent@example.com"),
		SubmissionDedupLockKey("other@example.com"),
	)
}

func TestSubmissionDedupLockKey_EmptyStringIsFnvOffsetBasis(t *testing.T) {
	t.Parallel()

	// FNV-1a offset basis for 64-bit; sanity check that the hash isn't
	// silently zero-initialised.
	assert.Equal(t, uint64(0xcbf29ce484222325), SubmissionDedupLockKey(""))
}
