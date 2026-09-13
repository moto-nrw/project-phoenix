package timezone

import (
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// WallClock aliases the canonical shared-kernel time of day.
type WallClock = calendar.WallClock

// NewWallClock constructs a validated time of day.
func NewWallClock(hour, minute, second, nanosecond int) (WallClock, error) {
	return calendar.NewWallClock(hour, minute, second, nanosecond)
}

// WallClockFromTime preserves only t's wall-clock components.
func WallClockFromTime(t time.Time) WallClock { return calendar.WallClockFromTime(t) }
