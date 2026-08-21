package schedule

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestResolveDayPlanningOnlyMarksAttendanceUnplannedWhenPlanSaysAbsent(t *testing.T) {
	t.Parallel()

	plannedTime := time.Date(2026, time.August, 21, 15, 0, 0, 0, time.UTC)
	planned := ResolveDayPlanning(DayPlanningInputs{
		HasActualAttendance: true,
		Pickup:              &EffectivePickupTime{PickupTime: &plannedTime},
	})
	assert.True(t, planned.ComesToday)
	assert.Equal(t, DayPlanningReasonPickupSchedule, planned.Reason)

	absent := ResolveDayPlanning(DayPlanningInputs{
		HasActualAttendance: true,
		Arrival:             &EffectiveArrivalTime{IsException: true},
	})
	assert.True(t, absent.ComesToday)
	assert.Equal(t, DayPlanningReasonUnplanned, absent.Reason)
}
