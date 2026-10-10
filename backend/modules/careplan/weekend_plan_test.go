package careplan_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
)

// A projection whose weekend follows Friday's plan (#3921) answers Saturday
// and Sunday with Friday's rows; without the setting the weekend has none.
func TestBaselineProjectionsReadFridayOnAFollowingWeekend(t *testing.T) {
	t.Parallel()

	saturday := calendar.NewDate(2026, time.September, 12)
	friday := &careplan.ArrivalSchedule{StudentID: 7, Weekday: 5}
	pickup := &careplan.PickupSchedule{StudentID: 7, Weekday: 5}
	arrivals := &careplan.ArrivalBaselineProjection{
		WeeklyByStudentDate: careplan.ArrivalPlansByStudent{7: {saturday: {5: friday}, saturday.AddDays(1): {5: friday}}},
	}
	pickups := &careplan.PickupBaselineProjection{
		WeeklyByStudentDate: careplan.PickupPlansByStudent{7: {saturday: {5: pickup}}},
	}
	assert.Nil(t, arrivals.ForDate(7, saturday))
	assert.Nil(t, pickups.ForDate(7, saturday))

	arrivals.WeekendFollowsFriday = true
	pickups.WeekendFollowsFriday = true
	assert.Same(t, friday, arrivals.ForDate(7, saturday))
	assert.Same(t, friday, arrivals.ForDate(7, saturday.AddDays(1)), "Sunday reads Friday too")
	assert.Same(t, pickup, pickups.ForDate(7, saturday))
}
