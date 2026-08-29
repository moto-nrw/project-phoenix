package timezone

import (
	"fmt"
	"time"
)

// WallClock is a time of day without a date or timezone. Its zero value means
// unset; midnight constructed through NewWallClock or WallClockFromTime is a
// distinct, valid value.
type WallClock struct {
	nanoseconds int64
	valid       bool
}

// NewWallClock constructs a WallClock from validated clock components.
func NewWallClock(hour, minute, second, nanosecond int) (WallClock, error) {
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 ||
		second < 0 || second > 59 || nanosecond < 0 || nanosecond >= int(time.Second) {
		return WallClock{}, fmt.Errorf(
			"invalid wall clock %02d:%02d:%02d.%09d",
			hour, minute, second, nanosecond,
		)
	}
	return WallClock{
		nanoseconds: int64(hour)*int64(time.Hour) +
			int64(minute)*int64(time.Minute) +
			int64(second)*int64(time.Second) + int64(nanosecond),
		valid: true,
	}, nil
}

// WallClockFromTime keeps only t's time-of-day components.
func WallClockFromTime(t time.Time) WallClock {
	clock, err := NewWallClock(t.Hour(), t.Minute(), t.Second(), t.Nanosecond())
	if err != nil {
		panic(err) // time.Time always exposes valid components.
	}
	return clock
}

// Time returns the time of day anchored to 0001-01-01 UTC for legacy
// time.Time callers and SQL TIME adapters.
func (c WallClock) Time() time.Time {
	if !c.valid {
		return time.Time{}
	}
	return time.Date(
		1, time.January, 1,
		c.Hour(), c.Minute(), c.Second(), c.Nanosecond(),
		time.UTC,
	)
}

// Hour returns the hour in the range 0..23.
func (c WallClock) Hour() int { return int(c.nanoseconds / int64(time.Hour)) }

// Minute returns the minute within the hour in the range 0..59.
func (c WallClock) Minute() int {
	return int(c.nanoseconds % int64(time.Hour) / int64(time.Minute))
}

// Second returns the second within the minute in the range 0..59.
func (c WallClock) Second() int {
	return int(c.nanoseconds % int64(time.Minute) / int64(time.Second))
}

// Nanosecond returns the nanosecond within the second.
func (c WallClock) Nanosecond() int { return int(c.nanoseconds % int64(time.Second)) }
