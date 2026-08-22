package sliceutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnique_Empty(t *testing.T) {
	t.Parallel()

	assert.Empty(t, Unique([]int64{}))
}

func TestUnique_NilInput(t *testing.T) {
	t.Parallel()

	assert.Nil(t, Unique[int64](nil))
}

func TestUnique_AllUnique(t *testing.T) {
	t.Parallel()

	input := []int64{1, 2, 3, 4, 5}
	assert.Equal(t, input, Unique(input))
}

func TestUnique_WithDuplicates(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []int64{1, 2, 3, 4, 5}, Unique([]int64{1, 2, 2, 3, 1, 4, 3, 5}))
}

func TestUnique_SingleElement(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []int64{42}, Unique([]int64{42}))
}

func TestUnique_AllSame(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []int64{7}, Unique([]int64{7, 7, 7, 7, 7}))
}

func TestUniquePositive_FiltersAndDedupes(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []int64{88, 99}, UniquePositive([]int64{0, 88, 88, -1, 99}))
	assert.Equal(t, []int64{50, 60}, UniquePositive([]int64{50, 0, 60, 50, -1}))
}

func TestUniquePositive_EmptyIsNil(t *testing.T) {
	t.Parallel()

	assert.Nil(t, UniquePositive(nil))
	assert.Nil(t, UniquePositive([]int64{}))
}
