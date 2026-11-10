package calendar

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// The summer sample crosses midnight only at Berlin's UTC+2 offset. This
// guards the calendar-day boundary independently of the host timezone.
func TestBerlinCalendarDatesUseSeasonalOffsets(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		instant time.Time
		date    string
		offset  int
	}{
		{"winter", time.Date(2026, time.January, 14, 22, 30, 0, 0, time.UTC), "2026-01-14", 3600},
		{"summer", time.Date(2026, time.July, 14, 22, 30, 0, 0, time.UTC), "2026-07-15", 7200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.date, DateFromTime(tc.instant).String())
			_, offset := tc.instant.In(Berlin).Zone()
			assert.Equal(t, tc.offset, offset)
		})
	}
}

// A failed zone lookup must stop initialization, not turn Berlin into a fixed
// UTC+1 zone. An unknown name triggers the same LoadLocation error path as a
// missing system zoneinfo database.
func TestTimezoneDataFailureStopsInitialization(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { mustLoadLocation("Missing/Berlin") })
}
