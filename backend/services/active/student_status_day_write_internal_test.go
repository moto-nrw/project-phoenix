package active

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDedupeStudentIDsPreservesFirstOccurrenceOrder(t *testing.T) {
	assert.Equal(t, []int64{7, 3, 9}, dedupeStudentIDs([]int64{7, 3, 7, 9, 3}))
	assert.Empty(t, dedupeStudentIDs(nil))
}
