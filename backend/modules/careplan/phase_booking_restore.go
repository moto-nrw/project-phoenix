package careplan

import (
	"context"
	"fmt"
	"slices"
)

// followPhaseOnRestore gives the restored bookings the bounds of their
// phase as it is now (#6657). A care end took the bookings with the bounds
// they had then; a phase edit in between moved only the bookings still in
// place. A bound that was the phase's at the care end, and is the outer
// bound of the application's bookings for the offering, takes the phase's
// current bound by the rules of a phase edit (ADR 0205): history stays, no
// bound moves before today. A dated switch, a care end and a hand-set bound
// keep their day.
//
// The bookings of the affected applications are locked and read before the
// decision, like the edit does. A capped booking restores only its end; a
// deleted one comes back with both bounds. A booking the phase no longer
// covers would come back empty: it stays as the care end left it, like any
// other row the restore cannot put back.
func (m *OfferingBookings) followPhaseOnRestore(ctx context.Context, restores []CareOfferingBookingRestore) ([]CareOfferingBookingRestore, error) {
	moves, err := restoreMoves(restores)
	if err != nil || len(moves) == 0 {
		return restores, err
	}
	childIDs := make([]int64, 0, len(moves))
	for childID := range moves {
		childIDs = append(childIDs, childID)
	}
	slices.Sort(childIDs)
	if err := m.engine.LockCareOfferingBookings(ctx, childIDs); err != nil {
		return nil, fmt.Errorf("lock bookings to restore: %w", err)
	}
	live, err := m.engine.CareOfferingBookingHistory(ctx, childIDs)
	if err != nil {
		return nil, fmt.Errorf("read bookings to restore: %w", err)
	}
	return withFollowedBounds(restores, restoredBookingBounds(live, restores, moves)), nil
}

// restoreMoves keys the phase moves of the restores by application. A move
// that leaves the bounds where they were moves nothing.
func restoreMoves(restores []CareOfferingBookingRestore) (map[int64]PhaseBoundsMove, error) {
	moves := map[int64]PhaseBoundsMove{}
	for _, restore := range restores {
		move := restore.PhaseBounds
		if move == nil || (move.StartBefore == move.StartAfter && move.EndBefore == move.EndAfter) {
			continue
		}
		if err := move.validate(); err != nil {
			return nil, err
		}
		childID := restore.Booking.RequestChildID
		if other, ok := moves[childID]; ok && other != *move {
			return nil, fmt.Errorf("restored bookings of request child %d carry different phase moves", childID)
		}
		moves[childID] = *move
	}
	return moves, nil
}

// withFollowedBounds writes the followed bounds into the restores and drops
// the restores they leave empty.
func withFollowedBounds(restores []CareOfferingBookingRestore, bounds map[int64]followedBooking) []CareOfferingBookingRestore {
	result := make([]CareOfferingBookingRestore, 0, len(restores))
	for _, restore := range restores {
		followed, ok := bounds[restore.Booking.ID]
		switch {
		case !ok:
		case followed.empty():
			continue
		case restore.WasDeleted:
			restore.Booking.ValidFrom, restore.Booking.ValidUntil = storedDay(followed.from), storedDay(followed.until)
		default:
			restore.Booking.ValidUntil = storedDay(followed.until)
		}
		result = append(result, restore)
	}
	return result
}

// restoredBookingBounds decides the followed bounds of the restored
// bookings. Each application is decided over its bookings as the restore
// would leave them; only restored bookings change, and a capped one keeps
// its live start.
func restoredBookingBounds(live []CareOfferingBooking, restores []CareOfferingBookingRestore, moves map[int64]PhaseBoundsMove) map[int64]followedBooking {
	perChild, deleted := bookingsAfterRestore(live, restores, moves)
	bounds := map[int64]followedBooking{}
	for childID, bookings := range perChild {
		for _, followed := range followPhaseBounds(bookings, moves[childID]) {
			wasDeleted, restoredRow := deleted[followed.booking.ID]
			if !restoredRow {
				continue
			}
			if !wasDeleted {
				// A capped row restores only its end; its start is live and
				// was moved by the edit already where it could.
				followed.from = bookingDay(followed.booking.ValidFrom)
			}
			bounds[followed.booking.ID] = followed
		}
	}
	return bounds
}

// bookingsAfterRestore are the bookings of the moved applications as the
// restore would leave them, by application and in id order: the live rows,
// a capped row with its restored end and a deleted row with its snapshot.
// The second map names the restored rows and whether they were deleted. A
// deleted row that is back already, or a capped row that is gone, is not
// restored and does not take part.
func bookingsAfterRestore(live []CareOfferingBooking, restores []CareOfferingBookingRestore, moves map[int64]PhaseBoundsMove) (map[int64][]CareOfferingBooking, map[int64]bool) {
	byID := map[int64]CareOfferingBooking{}
	for _, booking := range live {
		byID[booking.ID] = booking
	}
	restored := map[int64]bool{}
	for _, restore := range restores {
		if _, moved := moves[restore.Booking.RequestChildID]; !moved {
			continue
		}
		booking, exists := byID[restore.Booking.ID]
		if restore.WasDeleted == exists {
			continue
		}
		if restore.WasDeleted {
			booking = restore.Booking
		} else {
			booking.ValidUntil = restore.Booking.ValidUntil
		}
		byID[booking.ID] = booking
		restored[booking.ID] = restore.WasDeleted
	}
	perChild := map[int64][]CareOfferingBooking{}
	for _, booking := range byID {
		perChild[booking.RequestChildID] = append(perChild[booking.RequestChildID], booking)
	}
	for _, bookings := range perChild {
		slices.SortFunc(bookings, func(a, b CareOfferingBooking) int {
			switch {
			case a.ID < b.ID:
				return -1
			case a.ID > b.ID:
				return 1
			}
			return 0
		})
	}
	return perChild, restored
}
