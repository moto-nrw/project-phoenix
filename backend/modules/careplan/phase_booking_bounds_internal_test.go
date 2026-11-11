package careplan

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The phase in these cases runs from 2027-09-01 to 2028-07-31; a booking
// copied from it is [2027-09-01, 2028-08-01).
const (
	boundsStart calendar.Date = "2027-09-01"
	boundsEnd   calendar.Date = "2028-07-31"
	boundsToday calendar.Date = "2027-08-20"
)

func bound(day calendar.Date) *Date { return new(Date(day)) }

func booking(id, child, offering int64, from, until calendar.Date) CareOfferingBooking {
	b := CareOfferingBooking{ID: id, RequestChildID: child, CareOfferingID: offering}
	if from != "" {
		b.ValidFrom = bound(from)
	}
	if until != "" {
		b.ValidUntil = bound(until)
	}
	return b
}

func boundsMove(startDays, endDays int, today calendar.Date) PhaseBoundsMove {
	return PhaseBoundsMove{
		PhaseID:     1,
		StartBefore: boundsStart, StartAfter: boundsStart.AddDays(startDays),
		EndBefore: boundsEnd, EndAfter: boundsEnd.AddDays(endDays),
		Today: today,
	}
}

type movedBounds struct{ from, until calendar.Date }

func moved(t *testing.T, bookings []CareOfferingBooking, move PhaseBoundsMove) map[int64]movedBounds {
	t.Helper()
	changes, err := phaseBookingBoundChanges(bookings, move)
	require.NoError(t, err)
	result := map[int64]movedBounds{}
	for _, change := range changes {
		result[change.ID] = movedBounds{from: bookingDay(change.ValidFrom), until: bookingDay(change.ValidUntil)}
	}
	return result
}

// The bounds copied from the phase follow it in all four directions; the
// end stays exclusive (#6597).
func TestPhaseBookingBoundChanges_FollowsThePhase(t *testing.T) {
	t.Parallel()

	copied := []CareOfferingBooking{booking(1, 10, 100, boundsStart, boundsEnd.AddDays(1))}
	for name, tc := range map[string]struct {
		startDays, endDays int
		want               movedBounds
	}{
		"start earlier": {-7, 0, movedBounds{boundsStart.AddDays(-7), boundsEnd.AddDays(1)}},
		"start later":   {7, 0, movedBounds{boundsStart.AddDays(7), boundsEnd.AddDays(1)}},
		"end earlier":   {0, -14, movedBounds{boundsStart, boundsEnd.AddDays(-13)}},
		"end later":     {0, 14, movedBounds{boundsStart, boundsEnd.AddDays(15)}},
		"both":          {-7, 14, movedBounds{boundsStart.AddDays(-7), boundsEnd.AddDays(15)}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, map[int64]movedBounds{1: tc.want}, moved(t, copied, boundsMove(tc.startDays, tc.endDays, boundsToday)))
		})
	}
}

// A dated switch keeps its inner bound; only the outer bounds of the
// application's bookings of one offering follow. A bound set by hand or by a
// care end stays, and other applications move on their own.
func TestPhaseBookingBoundChanges_KeepsSwitchesAndOwnBounds(t *testing.T) {
	t.Parallel()

	switchDay := boundsStart.AddDays(30)
	bookings := []CareOfferingBooking{
		booking(1, 10, 100, boundsStart, switchDay),
		booking(2, 10, 100, switchDay, boundsEnd.AddDays(1)),
		booking(3, 11, 100, boundsStart, boundsStart.AddDays(60)),
		booking(4, 12, 100, boundsStart.AddDays(3), boundsEnd.AddDays(1)),
		booking(5, 13, 101, boundsStart, boundsEnd.AddDays(1)),
	}
	assert.Equal(t, map[int64]movedBounds{
		1: {boundsStart.AddDays(-7), switchDay},
		2: {switchDay, boundsEnd.AddDays(8)},
		3: {boundsStart.AddDays(-7), boundsStart.AddDays(60)},
		4: {boundsStart.AddDays(3), boundsEnd.AddDays(8)},
		5: {boundsStart.AddDays(-7), boundsEnd.AddDays(8)},
	}, moved(t, bookings, boundsMove(-7, 7, boundsToday)))
}

// A bound that equals the phase's old one but is no outer bound of the
// application's bookings of that offering is an inner switch bound, not the
// phase's: it stays.
func TestPhaseBookingBoundChanges_DateMatchAloneDoesNotMoveAnInnerBound(t *testing.T) {
	t.Parallel()

	bookings := []CareOfferingBooking{
		booking(1, 10, 100, boundsStart.AddDays(-10), boundsStart),
		booking(2, 10, 100, boundsStart, boundsEnd.AddDays(1)),
		booking(3, 10, 100, boundsEnd.AddDays(1), boundsEnd.AddDays(20)),
	}
	assert.Empty(t, moved(t, bookings, boundsMove(-7, 7, boundsToday)))
	open := []CareOfferingBooking{booking(4, 11, 100, "", boundsEnd.AddDays(1)), booking(5, 11, 100, boundsStart, "")}
	assert.Empty(t, moved(t, open, boundsMove(-7, 7, boundsToday)), "an open bound is the outer one")
}

// History stays: a start that has come and a booking that has ended keep
// their bounds, and no bound moves before today.
func TestPhaseBookingBoundChanges_KeepsHistory(t *testing.T) {
	t.Parallel()

	copied := []CareOfferingBooking{booking(1, 10, 100, boundsStart, boundsEnd.AddDays(1))}
	assert.Empty(t, moved(t, copied, boundsMove(-7, 0, boundsStart)), "a start that has come counts as executed")
	assert.Equal(t, map[int64]movedBounds{1: {boundsStart, boundsEnd.AddDays(8)}},
		moved(t, copied, boundsMove(-7, 7, boundsStart.AddDays(5))), "a running booking follows the end")
	assert.Empty(t, moved(t, copied, boundsMove(0, 7, boundsEnd.AddDays(1))), "an ended booking stays ended")

	assert.Equal(t, map[int64]movedBounds{1: {boundsToday, boundsEnd.AddDays(1)}},
		moved(t, copied, boundsMove(-30, 0, boundsToday)), "a start moved into the past begins today")
	running := boundsStart.AddDays(100)
	assert.Equal(t, map[int64]movedBounds{1: {boundsStart, running}},
		moved(t, copied, boundsMove(0, -300, running)), "an end moved into the past ends today")
}

// A move that would leave a booking empty refuses the whole edit.
func TestPhaseBookingBoundChanges_RefusesAnEmptyBooking(t *testing.T) {
	t.Parallel()

	switchDay := boundsStart.AddDays(30)
	split := []CareOfferingBooking{
		booking(1, 10, 100, boundsStart, switchDay),
		booking(2, 10, 100, switchDay, boundsEnd.AddDays(1)),
	}
	for name, move := range map[string]PhaseBoundsMove{
		"start past the switch": boundsMove(30, 0, boundsToday),
		"end before the switch": {PhaseID: 1, StartBefore: boundsStart, StartAfter: boundsStart, EndBefore: boundsEnd, EndAfter: switchDay.AddDays(-1), Today: boundsToday},
		"start past a care end": boundsMove(60, 0, boundsToday),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			bookings := split
			if name == "start past a care end" {
				bookings = []CareOfferingBooking{booking(3, 11, 100, boundsStart, boundsStart.AddDays(60))}
			}
			_, err := phaseBookingBoundChanges(bookings, move)
			var conflict *PhaseBookingBoundsConflictError
			require.True(t, errors.As(err, &conflict), "got %v", err)
		})
	}
}
