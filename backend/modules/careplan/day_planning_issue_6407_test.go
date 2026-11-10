package careplan

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Place before status (#6371, decision A in #6407): a child with an open
// check-in keeps the plan of its day. A sick note, excused absence or class
// trip is only a hint then; the planned pickup still applies, so the day reads
// "Abholplan heute" instead of "Kommt heute nicht" plus "ungeplant anwesend".
func TestResolveDayPlanningCheckedInChildKeepsPlanDespiteStatus(t *testing.T) {
	t.Parallel()

	pickupAt := time.Date(2026, time.September, 9, 15, 0, 0, 0, time.UTC)
	arrivalAt := time.Date(2026, time.September, 9, 11, 30, 0, 0, time.UTC)

	statuses := map[string]DayPlanningInputs{
		"sick":       {Sick: true},
		"excused":    {Excused: true},
		"class trip": {ClassTrip: true},
	}
	for name, status := range statuses {
		t.Run(name+" with planned pickup", func(t *testing.T) {
			in := status
			in.HasActualAttendance = true
			in.Pickup = &EffectivePickupTime{PickupTime: &pickupAt}
			got := ResolveDayPlanning(in)
			assert.True(t, got.ComesToday)
			assert.Equal(t, DayPlanningReasonPickupSchedule, got.Reason)
		})
		t.Run(name+" with planned arrival", func(t *testing.T) {
			in := status
			in.HasActualAttendance = true
			in.Arrival = &EffectiveArrivalTime{ArrivalTime: &arrivalAt}
			got := ResolveDayPlanning(in)
			assert.True(t, got.ComesToday)
			assert.Equal(t, DayPlanningReasonArrivalSchedule, got.Reason)
		})
		t.Run(name+" with timetable", func(t *testing.T) {
			in := status
			in.HasActualAttendance = true
			in.HasTimetable = true
			got := ResolveDayPlanning(in)
			assert.True(t, got.ComesToday)
			assert.Equal(t, DayPlanningReasonTimetable, got.Reason)
		})
	}
}

// Without a plan the checked-in child stays unplanned, and a timeless day
// exception still makes it unplanned: the status flags do not change that.
func TestResolveDayPlanningCheckedInChildWithoutPlanStaysUnplanned(t *testing.T) {
	t.Parallel()

	got := ResolveDayPlanning(DayPlanningInputs{HasActualAttendance: true, Sick: true})
	assert.True(t, got.ComesToday)
	assert.Equal(t, DayPlanningReasonUnplanned, got.Reason)

	got = ResolveDayPlanning(DayPlanningInputs{
		HasActualAttendance: true,
		Excused:             true,
		Pickup:              &EffectivePickupTime{IsException: true},
	})
	assert.True(t, got.ComesToday)
	assert.Equal(t, DayPlanningReasonUnplanned, got.Reason)
}

// A child without an open check-in keeps its status as the day's reason: the
// plan stays overruled exactly as before (#1458).
func TestResolveDayPlanningNotCheckedInChildKeepsStatus(t *testing.T) {
	t.Parallel()

	pickupAt := time.Date(2026, time.September, 9, 15, 0, 0, 0, time.UTC)
	cases := map[string]struct {
		in     DayPlanningInputs
		reason string
	}{
		"sick":       {DayPlanningInputs{Sick: true}, DayPlanningReasonSick},
		"excused":    {DayPlanningInputs{Excused: true}, DayPlanningReasonExcused},
		"class trip": {DayPlanningInputs{ClassTrip: true}, DayPlanningReasonClassTrip},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			in := tc.in
			in.Pickup = &EffectivePickupTime{PickupTime: &pickupAt}
			got := ResolveDayPlanning(in)
			assert.False(t, got.ComesToday)
			assert.Equal(t, tc.reason, got.Reason)
		})
	}
}
