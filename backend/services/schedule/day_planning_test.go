package schedule

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestResolveDayPlanning_Precedence(t *testing.T) {
	at := time.Date(2026, 5, 25, 8, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		in         DayPlanningInputs
		wantComes  bool
		wantReason string
	}{
		{
			name:       "actual attendance beats a reported absence",
			in:         DayPlanningInputs{HasActualAttendance: true, Sick: true},
			wantComes:  true,
			wantReason: DayPlanningReasonUnplanned,
		},
		{
			name:       "sick beats a planned arrival",
			in:         DayPlanningInputs{Sick: true, Arrival: &EffectiveArrivalTime{ArrivalTime: &at}},
			wantComes:  false,
			wantReason: DayPlanningReasonSick,
		},
		{
			name:       "class trip",
			in:         DayPlanningInputs{ClassTrip: true},
			wantComes:  false,
			wantReason: DayPlanningReasonClassTrip,
		},
		{
			name:       "excused",
			in:         DayPlanningInputs{Excused: true},
			wantComes:  false,
			wantReason: DayPlanningReasonExcused,
		},
		{
			name:       "planned arrival schedule",
			in:         DayPlanningInputs{Arrival: &EffectiveArrivalTime{ArrivalTime: &at}},
			wantComes:  true,
			wantReason: DayPlanningReasonArrivalSchedule,
		},
		{
			name:       "planned arrival exception",
			in:         DayPlanningInputs{Arrival: &EffectiveArrivalTime{ArrivalTime: &at, IsException: true}},
			wantComes:  true,
			wantReason: DayPlanningReasonArrivalException,
		},
		{
			name:       "arrival beats pickup",
			in:         DayPlanningInputs{Arrival: &EffectiveArrivalTime{ArrivalTime: &at}, Pickup: &EffectivePickupTime{PickupTime: &at}},
			wantComes:  true,
			wantReason: DayPlanningReasonArrivalSchedule,
		},
		{
			name:       "planned pickup schedule",
			in:         DayPlanningInputs{Pickup: &EffectivePickupTime{PickupTime: &at}},
			wantComes:  true,
			wantReason: DayPlanningReasonPickupSchedule,
		},
		{
			name:       "planned pickup exception",
			in:         DayPlanningInputs{Pickup: &EffectivePickupTime{PickupTime: &at, IsException: true}},
			wantComes:  true,
			wantReason: DayPlanningReasonPickupException,
		},
		{
			name:       "pickup beats timetable",
			in:         DayPlanningInputs{Pickup: &EffectivePickupTime{PickupTime: &at}, HasTimetable: true},
			wantComes:  true,
			wantReason: DayPlanningReasonPickupSchedule,
		},
		{
			name:       "timetable booking",
			in:         DayPlanningInputs{HasTimetable: true},
			wantComes:  true,
			wantReason: DayPlanningReasonTimetable,
		},
		{
			name:       "no plan",
			in:         DayPlanningInputs{},
			wantComes:  false,
			wantReason: DayPlanningReasonNoPlan,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveDayPlanning(tt.in)
			assert.Equal(t, tt.wantComes, got.ComesToday)
			assert.Equal(t, tt.wantReason, got.Reason)
		})
	}
}

// The whole-day cancellation (#1725/#1747): a timeless "Kommt heute nicht"
// exception on either leg cancels the day, and neither the other leg's regular
// schedule time, nor a timed exception on the other leg, nor a timetable
// booking may resurrect it — the parent portal resolves the same inputs to an
// absence. This is the precedence the care-day derivation relies on; weakening
// it makes the student search, the timetable, and the guardian tile disagree on
// the same child and date.
func TestResolveDayPlanning_TimelessExceptionCancelsDay(t *testing.T) {
	at := time.Date(2026, 5, 25, 8, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		in         DayPlanningInputs
		wantComes  bool
		wantReason string
	}{
		{
			name: "timeless arrival exception beats the pickup schedule",
			in: DayPlanningInputs{
				Arrival: &EffectiveArrivalTime{IsException: true},
				Pickup:  &EffectivePickupTime{PickupTime: &at},
			},
			wantComes:  false,
			wantReason: DayPlanningReasonArrivalException,
		},
		{
			name: "timeless pickup exception beats the arrival schedule",
			in: DayPlanningInputs{
				Arrival: &EffectiveArrivalTime{ArrivalTime: &at},
				Pickup:  &EffectivePickupTime{IsException: true},
			},
			wantComes:  false,
			wantReason: DayPlanningReasonPickupException,
		},
		{
			name: "timeless exception beats a timetable booking",
			in: DayPlanningInputs{
				Arrival:      &EffectiveArrivalTime{IsException: true},
				HasTimetable: true,
			},
			wantComes:  false,
			wantReason: DayPlanningReasonArrivalException,
		},
		{
			// The parent portal resolves pickup_absent || arrival_absent to an
			// absence before it looks at any time, so a timed exception on the
			// other leg must NOT resurrect the day here either.
			name: "timed exception on the other leg does not beat the cancellation",
			in: DayPlanningInputs{
				Arrival: &EffectiveArrivalTime{IsException: true},
				Pickup:  &EffectivePickupTime{PickupTime: &at, IsException: true},
			},
			wantComes:  false,
			wantReason: DayPlanningReasonArrivalException,
		},
		{
			name: "actual attendance still beats the cancellation",
			in: DayPlanningInputs{
				HasActualAttendance: true,
				Arrival:             &EffectiveArrivalTime{IsException: true},
			},
			wantComes:  true,
			wantReason: DayPlanningReasonUnplanned,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveDayPlanning(tt.in)
			assert.Equal(t, tt.wantComes, got.ComesToday)
			assert.Equal(t, tt.wantReason, got.Reason)
		})
	}
}

func TestResolveDayPlanning_NoTimeExceptionSurfacesNotes(t *testing.T) {
	arrival := ResolveDayPlanning(DayPlanningInputs{
		Arrival: &EffectiveArrivalTime{IsException: true, Notes: "Arzttermin"},
	})
	assert.False(t, arrival.ComesToday)
	assert.Equal(t, DayPlanningReasonArrivalException, arrival.Reason)
	assert.Equal(t, "Arzttermin", arrival.ExceptionNotes)

	pickup := ResolveDayPlanning(DayPlanningInputs{
		Pickup: &EffectivePickupTime{IsException: true, Notes: "Abgemeldet"},
	})
	assert.False(t, pickup.ComesToday)
	assert.Equal(t, DayPlanningReasonPickupException, pickup.Reason)
	assert.Equal(t, "Abgemeldet", pickup.ExceptionNotes)
}
