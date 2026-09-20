package careplan

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
)

func TestIsAtSchoolBeforeCheckIn(t *testing.T) {
	t.Parallel()

	expected := DayPlanningDecision{ComesToday: true, Reason: DayPlanningReasonArrivalSchedule}
	berlin := func(hour, minute int) time.Time {
		return time.Date(2026, time.September, 16, hour, minute, 0, 0, calendar.Berlin)
	}
	wallClock := func(hour, minute int) *time.Time {
		at := time.Date(0, time.January, 1, hour, minute, 0, 0, time.UTC)
		return &at
	}

	tests := []struct {
		name string
		in   AtSchoolInputs
		want bool
	}{
		{
			name: "expected child in the morning is at school",
			in:   AtSchoolInputs{Decision: expected, CareDayEnd: "18:00", Now: berlin(9, 30)},
			want: true,
		},
		{
			name: "any attendance record ends the state, a checkout too",
			in:   AtSchoolInputs{Decision: expected, CheckedInToday: true, CareDayEnd: "18:00", Now: berlin(9, 30)},
		},
		{
			name: "sick, excused, class trip and cancelled days resolve to not coming",
			in:   AtSchoolInputs{Decision: DayPlanningDecision{Reason: DayPlanningReasonSick}, CareDayEnd: "18:00", Now: berlin(9, 30)},
		},
		{
			name: "a child without a plan stays at home",
			in:   AtSchoolInputs{Decision: DayPlanningDecision{Reason: DayPlanningReasonNoPlan}, CareDayEnd: "18:00", Now: berlin(9, 30)},
		},
		{
			name: "unplanned attendance is never a waiting child",
			in:   AtSchoolInputs{Decision: DayPlanningDecision{ComesToday: true, Reason: DayPlanningReasonUnplanned}, CareDayEnd: "18:00", Now: berlin(9, 30)},
		},
		{
			name: "the pickup time ends the care day",
			in:   AtSchoolInputs{Decision: expected, PickupTime: wallClock(15, 0), CareDayEnd: "18:00", Now: berlin(15, 0)},
		},
		{
			name: "before the pickup time the child is still at school",
			in:   AtSchoolInputs{Decision: expected, PickupTime: wallClock(15, 0), CareDayEnd: "18:00", Now: berlin(14, 59)},
			want: true,
		},
		{
			name: "the pickup time wins over a later session end",
			in:   AtSchoolInputs{Decision: expected, PickupTime: wallClock(13, 0), CareDayEnd: "18:00", Now: berlin(16, 0)},
		},
		{
			name: "without a pickup time the session end closes the day",
			in:   AtSchoolInputs{Decision: expected, CareDayEnd: "17:00", Now: berlin(17, 0)},
		},
		{
			name: "the clock is read in Berlin, not UTC",
			in:   AtSchoolInputs{Decision: expected, CareDayEnd: "17:00", Now: berlin(16, 30).UTC()},
			want: true,
		},
		{
			name: "no end at all keeps the child at school",
			in:   AtSchoolInputs{Decision: expected, CareDayEnd: " ", Now: berlin(20, 0)},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, IsAtSchoolBeforeCheckIn(tc.in))
		})
	}
}
