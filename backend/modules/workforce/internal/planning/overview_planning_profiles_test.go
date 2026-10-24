package planning

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/require"
)

func TestWeeksWithShiftsExcludesCancelledOnlyWeeks(t *testing.T) {
	t.Parallel()
	weeks := weeksWithShifts([]*workforce.ShiftRecord{
		nil,
		{Date: calendar.Date("2026-10-05"), Cancelled: true},
		{Date: calendar.Date("2026-10-21")},
		{Date: calendar.Date("2026-10-14"), Cancelled: true},
		{Date: calendar.Date("2026-10-13")},
		{Date: calendar.Date("2026-10-20")},
	})
	require.Equal(t, []calendar.Date{"2026-10-12", "2026-10-19"}, weeks)
}
