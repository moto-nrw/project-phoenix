package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The down migration of 1.15.885 writes back exactly the booked windows of
// 1.15.878; the up migration changes that function only by the booked days
// (#4701).
func TestHolidayDayBookingRestoresTheBookedWindowsOf878(t *testing.T) {
	t.Parallel()
	start := strings.Index(holidayHostPlacedSQL, "CREATE OR REPLACE FUNCTION enrollment.holiday_host_booked_windows_for_retention")
	require.GreaterOrEqual(t, start, 0)
	assert.Equal(t, strings.TrimSpace(holidayHostPlacedSQL[start:]), strings.TrimSpace(holidayBookedWeekWindowsSQL))
	assert.Equal(t, 2, strings.Count(holidayBookedDayWindowsSQL, "booked_day.day"))
	assert.Contains(t, holidayBookedDayWindowsSQL, "booking.days")
	assert.Contains(t, holidayBookedDayWindowsSQL, "previous->'days'")
}
