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
