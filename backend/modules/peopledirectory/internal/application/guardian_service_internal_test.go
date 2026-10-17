package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The guardian service's unexported helpers. The forced-failure branches of
// the service run in the People Directory contract suite through its
// exported dependencies (#3755).

func TestSameInt64Set(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a, b []int64
		want bool
	}{
		{"equal same order", []int64{1, 2, 3}, []int64{1, 2, 3}, true},
		{"equal different order", []int64{3, 1, 2}, []int64{1, 2, 3}, true},
		{"both empty", []int64{}, []int64{}, true},
		{"different length", []int64{1, 2}, []int64{1, 2, 3}, false},
		{"same length different elements", []int64{1, 2, 3}, []int64{1, 2, 4}, false},
		{"duplicates differ", []int64{1, 1, 2}, []int64{1, 2, 2}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, sameInt64Set(tc.a, tc.b))
		})
	}
}
