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
	if len(moves) == 0 {
		return restores, nil
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
	restoredBounds := restoredBookingBounds(live, restores, moves)
	result := make([]CareOfferingBookingRestore, 0, len(restores))
	for _, restore := range restores {
		bounds, ok := restoredBounds[restore.Booking.ID]
		if !ok {
			result = append(result, restore)
			continue
		}
		if bounds.empty() {
			continue
		}
		if restore.WasDeleted {
			restore.Booking.ValidFrom = storedDay(bounds.from)
		}
		restore.Booking.ValidUntil = storedDay(bounds.until)
		result = append(result, restore)
	}
	return result, nil
}

// restoredBookingBounds decides the followed bounds of the restored
// bookings. Each application is decided over its bookings as the restore
// would leave them: the live rows, a capped row with its restored end and a
// deleted row with its snapshot. Only restored bookings change.
func restoredBookingBounds(live []CareOfferingBooking, restores []CareOfferingBookingRestore, moves map[int64]PhaseBoundsMove) map[int64]followedBooking {
	byID := map[int64]CareOfferingBooking{}
	for _, booking := range live {
		byID[booking.ID] = booking
	}
	restored, deleted := map[int64]bool{}, map[int64]bool{}
	for _, restore := range restores {
		if _, ok := moves[restore.Booking.RequestChildID]; !ok {
			continue
		}
		booking, exists := byID[restore.Booking.ID]
		switch {
		case restore.WasDeleted && !exists:
			booking = restore.Booking
		case !restore.WasDeleted && exists:
			booking.ValidUntil = restore.Booking.ValidUntil
		default:
			// A deleted row that is back already, or a capped row that is
			// gone, is not restored; it does not take part either.
			continue
		}
		byID[booking.ID] = booking
		restored[booking.ID], deleted[booking.ID] = true, restore.WasDeleted
	}
	perChild := map[int64][]CareOfferingBooking{}
	for _, booking := range byID {
		perChild[booking.RequestChildID] = append(perChild[booking.RequestChildID], booking)
	}
	bounds := map[int64]followedBooking{}
	for childID, bookings := range perChild {
		move, ok := moves[childID]
		if !ok {
			continue
		}
		slices.SortFunc(bookings, func(a, b CareOfferingBooking) int {
			switch {
			case a.ID < b.ID:
				return -1
			case a.ID > b.ID:
				return 1
			}
			return 0
		})
		for _, followed := range followPhaseBounds(bookings, move) {
			if !restored[followed.booking.ID] {
				continue
			}
			if !deleted[followed.booking.ID] {
				// A capped row restores only its end; its start is live and
				// was moved by the edit already where it could.
				followed.from = bookingDay(followed.booking.ValidFrom)
			}
			bounds[followed.booking.ID] = followed
		}
	}
	return bounds
}
