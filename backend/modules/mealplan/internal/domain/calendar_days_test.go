package domain

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
)

// Holiday periods become days only inside the window, across month ends,
// and closed days stay their own set (#3873).
func TestNewCalendarDaysClipsHolidayPeriodsToTheWindow(t *testing.T) {
	t.Parallel()
	from, to := calendar.NewDate(2026, time.July, 30), calendar.NewDate(2026, time.August, 3)
	days := NewCalendarDays(map[string]bool{"2026-07-31": true}, []HolidayPeriod{
		{Start: calendar.NewDate(2026, time.July, 20), End: calendar.NewDate(2026, time.July, 31)},
		{Start: calendar.NewDate(2026, time.August, 2), End: calendar.NewDate(2026, time.September, 1)},
	}, from, to)

	assert.Equal(t, map[Date]bool{calendar.NewDate(2026, time.July, 31): true}, days.Closed)
	assert.Equal(t, map[Date]bool{
		calendar.NewDate(2026, time.July, 30):  true,
		calendar.NewDate(2026, time.July, 31):  true,
		calendar.NewDate(2026, time.August, 2): true,
		calendar.NewDate(2026, time.August, 3): true,
	}, days.Holiday, "2026-08-01 lies between the periods")
}

// A period outside the window adds no day.
func TestNewCalendarDaysIgnoresPeriodsOutsideTheWindow(t *testing.T) {
	t.Parallel()
	day := calendar.NewDate(2026, time.September, 14)
	days := NewCalendarDays(nil, []HolidayPeriod{
		{Start: calendar.NewDate(2026, time.October, 1), End: calendar.NewDate(2026, time.October, 9)},
	}, day, day)
	assert.Empty(t, days.Holiday)
	assert.Empty(t, days.Closed)
}
