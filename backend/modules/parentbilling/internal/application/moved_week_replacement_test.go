package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// A booking is priced at most once, whatever the Büro later does to the
// dates of its week (#4699). The child was confirmed in week 1 (frozen at
// 45 €), then rebooked onto week 2, which waited. The Büro moved week 1 by a
// week afterwards; when week 2 moves up, the replacement names week 1 with
// its new dates, which no frozen line has. Matched by dates, week 2 would be
// priced on top of week 1; matched by the week's id, it replaces it.
func TestMovedUpReplacementOfAMovedWeekIsPricedOnce(t *testing.T) {
	t.Parallel()
	confirmed := chainChild(1, 1, 4500)
	frozen := computeHolidayContribution([]parentbilling.HolidayChild{confirmed}, holidayRules{}, false, nil)
	require.Equal(t, int64(4500), frozen.TotalCents)
	stored := parentbilling.HolidayContribution{Children: frozen.Children, Frozen: true}

	moved := confirmed.Bookings[0].WeekStartsOn.AddDays(7)
	current := chainChild(1, 1, 0, 4500)
	current.Bookings = current.Bookings[1:]
	current.Bookings[0].Promoted = true
	current.Bookings[0].ReplacedWeeks = []parentbilling.HolidayBookingReplacement{{
		WeekID: confirmed.Bookings[0].WeekID, WeekStartsOn: moved, WeekEndsOn: moved.AddDays(4),
	}}
	member := siblingMember{
		registration: parentbilling.HolidayRegistration{ID: 9, Status: parentbilling.HolidayRegistrationConfirmed, Children: []parentbilling.HolidayChild{current}},
		frozen:       &stored, movedUp: true,
	}
	result := computeHolidaySiblings([]holidayMember{member.chainMember()}, holidayRules{})[0]
	assert.Equal(t, int64(4500), result.TotalCents, "the replacement keeps the one frozen price, it is not billed again")
}

// A frozen week prices one booking only (#4699). The child was confirmed in
// week 1 (45 €), rebooked onto week 2, which waited, and then booked week 1
// again, which waited too. When both move up, week 1 takes its frozen price
// and week 2 can no longer claim it as its replacement: 90 €, not 45 €.
func TestFrozenWeekPricesOneMovedUpBookingOnly(t *testing.T) {
	t.Parallel()
	confirmed := chainChild(1, 1, 4500)
	frozen := computeHolidayContribution([]parentbilling.HolidayChild{confirmed}, holidayRules{}, false, nil)
	stored := parentbilling.HolidayContribution{Children: frozen.Children, Frozen: true}

	current := chainChild(1, 1, 4500, 4500)
	current.Bookings[0].Promoted = true
	current.Bookings[1].Promoted = true
	current.Bookings[1].ReplacedWeeks = []parentbilling.HolidayBookingReplacement{{
		WeekID: confirmed.Bookings[0].WeekID, WeekStartsOn: confirmed.Bookings[0].WeekStartsOn, WeekEndsOn: confirmed.Bookings[0].WeekEndsOn,
	}}
	member := siblingMember{
		registration: parentbilling.HolidayRegistration{ID: 9, Status: parentbilling.HolidayRegistrationConfirmed, Children: []parentbilling.HolidayChild{current}},
		frozen:       &stored, movedUp: true,
	}
	result := computeHolidaySiblings([]holidayMember{member.chainMember()}, holidayRules{})[0]
	assert.Equal(t, int64(9000), result.TotalCents, "both placed weeks are billed, each once")
}

// A direct replacement holds the frozen price of the week it replaced
// (#4699). The child was confirmed in week 1 (45 €), rebooked directly onto
// week 2, which had a place, and then booked week 1 again, which waited.
// When week 1 moves up, week 2 keeps the frozen 45 € and week 1 comes anew:
// 90 €, not 45 €.
func TestDirectReplacementKeepsItsFrozenWeekWhenTheOldWeekMovesUp(t *testing.T) {
	t.Parallel()
	confirmed := chainChild(1, 1, 4500)
	frozen := computeHolidayContribution([]parentbilling.HolidayChild{confirmed}, holidayRules{}, false, nil)
	stored := parentbilling.HolidayContribution{Children: frozen.Children, Frozen: true}

	current := chainChild(1, 1, 4500, 4500)
	current.Bookings[0].Promoted = true
	current.Bookings[1].ReplacedWeeks = []parentbilling.HolidayBookingReplacement{{
		WeekID: confirmed.Bookings[0].WeekID, WeekStartsOn: confirmed.Bookings[0].WeekStartsOn, WeekEndsOn: confirmed.Bookings[0].WeekEndsOn,
	}}
	member := siblingMember{
		registration: parentbilling.HolidayRegistration{ID: 9, Status: parentbilling.HolidayRegistrationConfirmed, Children: []parentbilling.HolidayChild{current}},
		frozen:       &stored, movedUp: true,
	}
	result := computeHolidaySiblings([]holidayMember{member.chainMember()}, holidayRules{})[0]
	assert.Equal(t, int64(9000), result.TotalCents, "the week that moved up is billed besides the direct replacement")
}
