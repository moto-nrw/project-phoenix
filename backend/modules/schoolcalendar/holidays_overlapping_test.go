package schoolcalendar

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHolidaysOverlappingSelectsActiveHolidaysInTheWindow(t *testing.T) {
	t.Parallel()
	assert.Equal(t, CalendarPeriodFilter{
		PeriodType: PeriodTypeHoliday, ActiveOnly: true, OverlappingFrom: "2026-07-01", OverlappingTo: "2026-08-31",
	}, HolidaysOverlapping("2026-07-01", "2026-08-31"))
}
