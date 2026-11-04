package schedulerecords

import (
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// SetWallClockTimes records explicit TIME values, including midnight. Legacy
// callers may still provide non-zero time.Time values directly.
func (i *ActivityInstance) SetWallClockTimes(start, end time.Time) {
	i.StartTime, i.EndTime = calendar.NormalizeWallClock(start), calendar.NormalizeWallClock(end)
	i.ClockTimesSet = true
}

// SetStartClock records an explicit TIME value, not an optional instant.
func (t *Timeframe) SetStartClock(start time.Time) {
	t.StartTime = calendar.NormalizeWallClock(start)
	t.StartTimeSet = true
	if t.EndTime != nil {
		end := calendar.NormalizeWallClock(*t.EndTime)
		t.EndTime = &end
	}
}
