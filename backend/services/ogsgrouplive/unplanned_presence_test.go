package ogsgrouplive

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	activeService "github.com/moto-nrw/project-phoenix/services/active"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

func TestApplyPlanningTracksUnplannedPresenceUntilCheckout(t *testing.T) {
	t.Parallel()

	const (
		absentID     int64 = 101
		presentID    int64 = 102
		checkedOutID int64 = 103
		plannedID    int64 = 104
		sickID       int64 = 105
	)
	checkIn := time.Date(2026, time.August, 21, 8, 0, 0, 0, time.UTC)
	checkOut := checkIn.Add(time.Hour)
	arrivalException := &scheduleService.EffectiveArrivalTime{IsException: true}
	state := &buildState{
		projected: []Student{
			{ID: absentID, fullAccess: true},
			{ID: presentID, fullAccess: true},
			{ID: checkedOutID, fullAccess: true},
			{ID: plannedID, fullAccess: true},
			{ID: sickID, Sick: true, fullAccess: true},
		},
		data: &snapshot{locations: &activeService.StudentLocationSnapshot{
			Attendances: map[int64]*activeService.AttendanceStatus{
				presentID: {
					Status:      "checked_in",
					CheckInTime: &checkIn,
				},
				checkedOutID: {
					Status:       "checked_out",
					CheckInTime:  &checkIn,
					CheckOutTime: &checkOut,
				},
				plannedID: {
					Status:      "checked_in",
					CheckInTime: &checkIn,
				},
				sickID: {
					Status:      "checked_in",
					CheckInTime: &checkIn,
				},
			},
		}},
		arrivals: map[int64]*scheduleService.EffectiveArrivalTime{
			absentID:     arrivalException,
			presentID:    arrivalException,
			checkedOutID: arrivalException,
		},
		pickups: map[int64]*scheduleService.EffectivePickupTime{
			plannedID: {PickupTime: &checkOut},
		},
		timetable: map[int64]struct{}{},
	}

	applyPlanning(state, nil)

	assert.Equal(t, "arrival_exception", state.projected[0].DayPlanningReason)
	assert.Equal(t, "unplanned_attendance", state.projected[1].DayPlanningReason)
	assert.Equal(t, "arrival_exception", state.projected[2].DayPlanningReason)
	assert.Equal(t, "pickup_schedule", state.projected[3].DayPlanningReason)
	assert.Equal(t, "unplanned_attendance", state.projected[4].DayPlanningReason)
}
