package careplan

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The restore decides the followed bounds over the application's bookings
// as the restore leaves them (#6657): a capped row with its restored end, a
// deleted row with its snapshot. Only restored rows change, a capped row
// keeps its live start, and a row the phase no longer covers is empty.
func TestRestoredBookingBounds(t *testing.T) {
	t.Parallel()

	careEnd, switchDay := calendar.Date("2027-10-01"), calendar.Date("2028-02-01")
	move := PhaseBoundsMove{PhaseID: 1, StartBefore: boundsStart, StartAfter: boundsStart.AddDays(-7),
		EndBefore: boundsEnd, EndAfter: boundsEnd.AddDays(14), Today: boundsToday}
	live := []CareOfferingBooking{booking(10, 5, 7, boundsStart.AddDays(-7), careEnd)}
	restores := []CareOfferingBookingRestore{
		{Booking: booking(10, 5, 7, boundsStart, switchDay), PhaseBounds: &move},
		{Booking: booking(11, 5, 7, switchDay, boundsEnd.AddDays(1)), WasDeleted: true, PhaseBounds: &move},
	}

	bounds := restoredBookingBounds(live, restores, map[int64]PhaseBoundsMove{5: move})
	_, innerMoved := bounds[10]
	assert.False(t, innerMoved, "the switch day of the capped half is no phase bound")
	assert.Equal(t, followedBooking{booking: booking(11, 5, 7, switchDay, boundsEnd.AddDays(1)), from: switchDay, until: boundsEnd.AddDays(15)}, bounds[11],
		"the deleted half takes the current phase end and keeps its switch day")

	shorter := move
	shorter.EndAfter = switchDay.AddDays(-10)
	bounds = restoredBookingBounds(live, restores, map[int64]PhaseBoundsMove{5: shorter})
	assert.True(t, bounds[11].empty(), "a half the phase no longer covers would be empty")

	capped := []CareOfferingBookingRestore{{Booking: booking(10, 5, 7, boundsStart, boundsEnd.AddDays(1)), PhaseBounds: &move}}
	bounds = restoredBookingBounds(live, capped, map[int64]PhaseBoundsMove{5: move})
	assert.Equal(t, boundsStart.AddDays(-7), bounds[10].from, "a capped row keeps its live start")
	assert.Equal(t, boundsEnd.AddDays(15), bounds[10].until, "a capped row takes the current phase end")
}
