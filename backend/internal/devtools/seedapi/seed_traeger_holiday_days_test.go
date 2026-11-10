package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The Winterferien booked by days lie on a Monday-to-Friday week whose
// registration window is still open for more than a week (#4701).
func TestSeedHolidayDayPeriodLiesAheadOfItsRegistration(t *testing.T) {
	t.Parallel()
	for _, today := range []calendar.Date{calendar.NewDate(2026, 11, 10), calendar.NewDate(2027, 1, 30), calendar.NewDate(2027, 2, 3)} {
		monday := seedNextWinterMonday(today)
		assert.Equal(t, time.Monday, monday.Weekday())
		assert.Equal(t, time.February, monday.Month())
		assert.True(t, monday.AddDays(-3).After(today.AddDays(7)), "%s leaves the window open after %s", monday, today)
	}
	period := seedHolidayDayPeriod(calendar.NewDate(2026, 11, 10), calendar.NewDate(2027, 2, 1), 31)
	assert.Equal(t, 3, period["max_days_per_child"])
	week := period["weeks"].([]map[string]any)[0]
	assert.Equal(t, "day", week["booking_unit"])
	assert.Equal(t, "2027-02-05", week["ends_on"])
}
