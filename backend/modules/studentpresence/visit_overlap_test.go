package studentpresence

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestVisitsOverlapEmptyIntervals(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)
	middle, end := start.Add(time.Hour), start.Add(2*time.Hour)
	empty := Visit{EntryTime: middle, ExitTime: &middle}
	for _, other := range []Visit{
		{EntryTime: start, ExitTime: &end},
		{EntryTime: start},
		empty,
	} {
		assert.False(t, VisitsOverlap(empty, other))
		assert.False(t, VisitsOverlap(other, empty))
	}
}
