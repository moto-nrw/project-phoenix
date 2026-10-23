package departure

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// DayRules is the per-weekday rule both class lists print (#4749): every
// weekday gets an entry, a day without a rule reads "geht alleine", several
// modes join with " / ", and an accompanied day names the companions of that
// day before it falls back to the free-text note.
func TestDayRules(t *testing.T) {
	t.Parallel()

	allowed := AllowedDepartureModes{
		PickupDayMonday:    {DeparturePickup},
		PickupDayTuesday:   {DepartureBus, DeparturePickup},
		PickupDayWednesday: {DepartureAccompanied},
		PickupDayThursday:  {DepartureAccompanied},
	}
	companions := []CompanionLink{{CompanionStudentID: 7, FirstName: "Mia", LastName: "Schulz", Weekdays: []string{PickupDayWednesday}}}

	got := DayRules(allowed, nil, "  Oma Erika  ", companions)

	assert.Equal(t, map[string]string{
		PickupDayMonday:    "wird abgeholt",
		PickupDayTuesday:   "fährt Bus / wird abgeholt",
		PickupDayWednesday: "mit anderem Kind (mit: Mia Schulz)",
		PickupDayThursday:  "mit anderem Kind (mit: Oma Erika)",
		PickupDayFriday:    "geht alleine",
	}, got)
}

// Without explicit allowed modes the legacy exclusive day plan answers; a
// companion link on a day the plan does not allow "Anderes Kind" is not
// printed.
func TestDayRulesFallsBackToDepartureDays(t *testing.T) {
	t.Parallel()

	fallback := DepartureDays{PickupDayMonday: DepartureBus}
	companions := []CompanionLink{{CompanionStudentID: 7, FirstName: "Mia", Weekdays: []string{PickupDayMonday}}}

	got := DayRules(nil, fallback, "", companions)

	assert.Equal(t, "fährt Bus", got[PickupDayMonday])
	assert.Equal(t, "geht alleine", got[PickupDayTuesday])
	assert.Len(t, got, len(PickupDayOrder))
}
