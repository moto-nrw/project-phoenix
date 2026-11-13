package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
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

// The Winterferien by criteria lie a week after those booked by days, with
// their registration window still open, and give no place when sending
// (#4700).
func TestSeedHolidayCriteriaPeriodAllocatesByCriteria(t *testing.T) {
	t.Parallel()
	today := calendar.NewDate(2026, 11, 10)
	monday := seedNextWinterMonday(today).AddDays(7)
	period := seedHolidayCriteriaPeriod(today, monday, 31)
	assert.Equal(t, "criteria", period["allocation_mode"])
	week := period["weeks"].([]map[string]any)[0]
	assert.Equal(t, "2027-02-08", week["starts_on"])
	assert.True(t, monday.AddDays(-10).After(today))
}
