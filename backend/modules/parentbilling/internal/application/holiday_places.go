package application

import (
	"context"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// Places of the holiday weeks (#4699, decision E-e). A booking on the
// waiting list costs nothing. When it moves up, bindingly, the contribution
// of its registration is computed again in the same transaction, if the
// registration is confirmed: the base prices it froze stay, the newly placed
// weeks come at their current price, and its sibling group is ranked again.
// An issued invoice keeps its positions, like after a cancellation of a
// sibling (#5662); the page of the registration shows the difference.
//
// A move up happens in Enrollment under the locks of a cancellation, a
// rebooking or a new capacity. These run here, under the lock of the
// period's invoices first, so the release of a billing run sees either the
// state before or after, never a placement without its contribution.

// withMovedUpBookings adds to the frozen children the placed bookings of the
// registration that the frozen contribution does not price yet.
func withMovedUpBookings(frozen, current []parentbilling.HolidayChild) []parentbilling.HolidayChild {
	byID := make(map[int64]int, len(frozen))
	for i, child := range frozen {
		byID[child.ID] = i
	}
	for _, child := range current {
		i, ok := byID[child.ID]
		if !ok {
			continue
		}
		for _, booking := range child.Bookings {
			if booking.Waitlisted || slices.ContainsFunc(frozen[i].Bookings, func(priced parentbilling.HolidayBooking) bool {
				return priced.WeekStartsOn == booking.WeekStartsOn && priced.WeekEndsOn == booking.WeekEndsOn
			}) {
				continue
			}
			frozen[i].Bookings = append(frozen[i].Bookings, booking)
		}
	}
	return frozen
}

// refreezeMovedUp computes the contribution of every registration that moved
// up again, with its sibling group; only confirmed ones keep a new version. The caller holds the
// invoices of the period; issued names the registrations an issued invoice
// bills, which keep their contribution.
func (h *HolidayContributions) refreezeMovedUp(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID int64, movedUp []int64, issued map[int64]bool) error {
	done := map[int64]bool{}
	for _, registrationID := range movedUp {
		if done[registrationID] {
			continue
		}
		registration, err := h.registrations.HolidayRegistration(ctx, reviewer, periodID, registrationID)
		if err != nil {
			return err
		}
		members, _, err := h.siblingsOf(ctx, reviewer, periodID, registration)
		if err != nil {
			return err
		}
		for i := range members {
			done[members[i].id()] = true
			members[i].movedUp = slices.Contains(movedUp, members[i].id())
		}
		// A received registration freezes nothing yet, but a child of it
		// that now has a place can change the ranks of its confirmed
		// siblings.
		if err := h.refreeze(ctx, reviewer, periodID, members, issued, true); err != nil {
			return err
		}
	}
	return nil
}

// RebookHolidayChild holds the invoices of the period and the period,
// rebooks the child through Enrollment and computes the contribution of
// every confirmed registration that moved up again.
func (h *HolidayContributions) RebookHolidayChild(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID, registrationID, childID int64,
	bookings []parentbilling.HolidayBookingChoice,
) (parentbilling.HolidayPlaceChange, error) {
	if reviewer.OrganizationID <= 0 || reviewer.AccountID <= 0 {
		return parentbilling.HolidayPlaceChange{}, errTraegerRequired
	}
	_, issued, err := h.lockSiblings(ctx, reviewer, periodID)
	if err != nil {
		return parentbilling.HolidayPlaceChange{}, err
	}
	registration, err := h.registrations.RebookHolidayChild(ctx, reviewer, periodID, registrationID, childID, bookings)
	if err != nil {
		return parentbilling.HolidayPlaceChange{}, err
	}
	if err := h.refreezeMovedUp(ctx, reviewer, periodID, registration.MovedUp, issued); err != nil {
		return parentbilling.HolidayPlaceChange{}, err
	}
	return parentbilling.HolidayPlaceChange{
		Registration: registration.View, RegistrationID: registration.ID, MovedUp: movedUpOrEmpty(registration.MovedUp),
	}, nil
}

// SetHolidayWeekCapacity holds the invoices of the period and the period,
// sets the places of the week through Enrollment and computes the
// contribution of every confirmed registration that moved up again.
func (h *HolidayContributions) SetHolidayWeekCapacity(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID, weekID int64, capacity *int) (parentbilling.HolidayPlaceChange, error) {
	if reviewer.OrganizationID <= 0 || reviewer.AccountID <= 0 {
		return parentbilling.HolidayPlaceChange{}, errTraegerRequired
	}
	_, issued, err := h.lockSiblings(ctx, reviewer, periodID)
	if err != nil {
		return parentbilling.HolidayPlaceChange{}, err
	}
	movedUp, err := h.registrations.SetHolidayWeekCapacity(ctx, reviewer, periodID, weekID, capacity)
	if err != nil {
		return parentbilling.HolidayPlaceChange{}, err
	}
	if err := h.refreezeMovedUp(ctx, reviewer, periodID, movedUp, issued); err != nil {
		return parentbilling.HolidayPlaceChange{}, err
	}
	return parentbilling.HolidayPlaceChange{WeekID: weekID, Capacity: capacity, MovedUp: movedUpOrEmpty(movedUp)}, nil
}

func movedUpOrEmpty(ids []int64) []int64 {
	if ids == nil {
		return []int64{}
	}
	return slices.Compact(slices.Sorted(slices.Values(ids)))
}
