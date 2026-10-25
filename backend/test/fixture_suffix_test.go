package test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFixtureSuffixesAreUniqueUnderParallelAllocation(t *testing.T) {
	t.Parallel()
	const workers, perWorker = 64, 1024
	results := make(chan []int64, workers)
	for range workers {
		go func() {
			values := make([]int64, perWorker)
			for i := range values {
				values[i] = uniqueFixtureSuffix()
			}
			results <- values
		}()
	}
	values := make([]int64, 0, workers*perWorker)
	for range workers {
		values = append(values, <-results...)
	}
	slices.Sort(values)
	for i := 1; i < len(values); i++ {
		require.NotEqual(t, values[i-1], values[i], "parallel fixtures must never share a suffix")
	}
}
