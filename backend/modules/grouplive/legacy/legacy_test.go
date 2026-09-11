package legacy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/grouplive"
)

func TestNewRejectsMissingSourcesAtComposition(t *testing.T) {
	t.Parallel()

	query, err := New(Sources{})

	assert.ErrorIs(t, err, ErrIncompleteSources, "missing wiring fails composition, not the first request")
	assert.Nil(t, query)
}

func TestSortedRecordsUsesGermanCollation(t *testing.T) {
	t.Parallel()

	records := sortedRecords([]grouplive.GroupRecord{{ID: 1, Name: "Zebra"}, {ID: 2, Name: "Äpfel"}, {ID: 3, Name: "apfel"}})

	assert.Equal(t, []string{"apfel", "Äpfel", "Zebra"}, []string{records[0].Name, records[1].Name, records[2].Name},
		"umlauts sort with their base letter, ahead of Z and after the plain letter (DIN 5007-1)")
}

// TestDecideDayFollowsTheSharedPrecedence pins the projection to the same
// day-planning rules the student search and the timetable use: actual
// presence turns an absent decision into unplanned attendance, but a
// timeless exception and a plan with a time keep their own reasons.
func TestDecideDayFollowsTheSharedPrecedence(t *testing.T) {
	t.Parallel()

	pickupAt := time.Date(2026, time.August, 21, 15, 0, 0, 0, time.UTC)
	exception := &grouplive.Arrival{IsException: true, Notes: "Zahnarzt"}

	cases := []struct {
		name   string
		inputs grouplive.DayInputs
		want   grouplive.DayDecision
	}{
		{"absent with exception", grouplive.DayInputs{Arrival: exception},
			grouplive.DayDecision{Reason: grouplive.DayReasonArrivalException, ExceptionNotes: "Zahnarzt"}},
		{"present despite exception", grouplive.DayInputs{Present: true, Arrival: exception},
			grouplive.DayDecision{ComesToday: true, Reason: grouplive.DayReasonUnplanned}},
		{"checked out keeps the exception", grouplive.DayInputs{Arrival: exception},
			grouplive.DayDecision{Reason: grouplive.DayReasonArrivalException, ExceptionNotes: "Zahnarzt"}},
		{"pickup schedule", grouplive.DayInputs{Present: true, Pickup: &grouplive.Pickup{PickupTime: &pickupAt}},
			grouplive.DayDecision{ComesToday: true, Reason: grouplive.DayReasonPickupSchedule}},
		{"sick but present", grouplive.DayInputs{Present: true, Sick: true, HasTimetable: true},
			grouplive.DayDecision{ComesToday: true, Reason: grouplive.DayReasonUnplanned}},
		{"timetable", grouplive.DayInputs{HasTimetable: true},
			grouplive.DayDecision{ComesToday: true, Reason: grouplive.DayReasonTimetable}},
		{"no plan", grouplive.DayInputs{}, grouplive.DayDecision{Reason: grouplive.DayReasonNoPlan}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, planning{}.DecideDay(tc.inputs))
		})
	}
}

func TestCalendarRendersBerlinWallClock(t *testing.T) {
	t.Parallel()

	fixed := time.Date(2026, time.August, 21, 6, 5, 0, 0, time.UTC)
	c := calendar{now: func() time.Time { return fixed }}

	assert.Equal(t, grouplive.Date("2026-08-21"), c.Today())
	assert.Equal(t, "08:05", c.Clock(fixed), "instants render in Berlin summer time")
}
