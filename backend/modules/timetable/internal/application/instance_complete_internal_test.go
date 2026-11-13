package application

import (
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
)

// TestReopenDeadline covers #5676: a block ended before its planned end stays
// reopenable until then; otherwise the five minutes after completion apply.
func TestReopenDeadline(t *testing.T) {
	t.Parallel()

	berlinAt := func(hour, minute int) time.Time {
		return time.Date(2026, 8, 13, hour, minute, 0, 0, calendar.Berlin)
	}
	plannedEnd := berlinAt(15, 0)

	tests := []struct {
		name        string
		spontaneous bool
		completedAt time.Time
		want        time.Time
	}{
		{name: "ended early keeps the planned end", completedAt: berlinAt(14, 10), want: plannedEnd},
		{name: "ended shortly before the end keeps five minutes", completedAt: berlinAt(14, 58), want: berlinAt(15, 3)},
		{name: "ended after the planned end keeps five minutes", completedAt: berlinAt(15, 20), want: berlinAt(15, 25)},
		{name: "spontaneous block keeps five minutes", spontaneous: true, completedAt: berlinAt(14, 10), want: berlinAt(14, 15)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			instance := plannedLifecycleInstance()
			instance.IsSpontaneous = tt.spontaneous
			assert.True(t, tt.want.Equal(reopenDeadline(instance, tt.completedAt)), "got %s", reopenDeadline(instance, tt.completedAt))
		})
	}
}
