package application

import (
	"context"
	"fmt"
	"slices"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
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
// rebooking, a new capacity or a decision by criteria. The cancellation runs
// here; the other three run in the holiday allocation workflow (ADR 0206).
// Both take the lock of the period's invoices first, so the release of a
// billing run sees either the state before or after, never a placement
// without its contribution.

// withMovedUpBookings adds to the frozen children the placed bookings of the
// registration that actually moved up and the frozen contribution does
// not price yet. Direct rebookings keep their frozen contribution. Each
// frozen booking prices one current booking only: a week priced directly
// is no longer there for a replacement of it, so the replacement is added.
func withMovedUpBookings(frozen, current []parentbilling.HolidayChild) []parentbilling.HolidayChild {
	byID := make(map[int64]int, len(frozen))
	for i, child := range frozen {
		byID[child.ID] = i
	}
	for _, child := range current {
		if i, ok := byID[child.ID]; ok {
			frozen[i].Bookings = append(frozen[i].Bookings, movedUpBookings(frozen[i].Bookings, child.Bookings)...)
		}
	}
	return frozen
}

// movedUpBookings are the moved-up bookings of current that no frozen
// booking prices. Bookings placed directly claim their frozen booking first,
// by their own week, then by a week they replaced; moved-up bookings claim
// what is left the same way. A moved-up week whose frozen booking a direct
// replacement holds is therefore priced anew.
func movedUpBookings(frozen, current []parentbilling.HolidayBooking) []parentbilling.HolidayBooking {
	m := frozenClaims{frozen: frozen, used: make([]bool, len(frozen))}
	var added []parentbilling.HolidayBooking
	for _, promoted := range []bool{false, true} {
		var open []parentbilling.HolidayBooking
		for _, booking := range current {
			if !booking.Waitlisted && booking.Promoted == promoted && !m.claim(booking.WeekID, booking.WeekStartsOn, booking.WeekEndsOn) {
				open = append(open, booking)
			}
		}
		for _, booking := range open {
			if !m.claimReplaced(booking) && promoted {
				added = append(added, booking)
			}
		}
	}
	return added
}

// frozenClaims hands out each frozen booking once.
type frozenClaims struct {
	frozen []parentbilling.HolidayBooking
	used   []bool
}

func (m frozenClaims) claim(weekID int64, startsOn, endsOn calendar.Date) bool {
	for k, priced := range m.frozen {
		if !m.used[k] && pricesHolidayWeek(priced, weekID, startsOn, endsOn) {
			m.used[k] = true
			return true
		}
	}
	return false
}

func (m frozenClaims) claimReplaced(booking parentbilling.HolidayBooking) bool {
	return slices.ContainsFunc(booking.ReplacedWeeks, func(previous parentbilling.HolidayBookingReplacement) bool {
		return m.claim(previous.WeekID, previous.WeekStartsOn, previous.WeekEndsOn)
	})
}

// pricesHolidayWeek tells whether a frozen booking prices the week: by its id
// when the frozen line names one, so a later change of the week's dates
// cannot make it look unpriced; by its dates for a line frozen without one.
func pricesHolidayWeek(priced parentbilling.HolidayBooking, weekID int64, startsOn, endsOn calendar.Date) bool {
	if priced.WeekID != 0 && weekID != 0 {
		return priced.WeekID == weekID
	}
	return priced.WeekStartsOn == startsOn && priced.WeekEndsOn == endsOn
}

// refreezeMovedUp computes the contribution of every registration that moved
// up again, with its sibling group; only confirmed ones keep a new version. The caller holds the
// invoices of the period. Issued invoices keep their positions, while the
// new contribution exposes any correction needed.
func (h *HolidayContributions) refreezeMovedUp(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID int64, movedUp []int64) error {
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
		if err := h.refreeze(ctx, reviewer, periodID, members, nil, true); err != nil {
			return err
		}
	}
	return nil
}

// HoldHolidayAllocationInvoices takes the billing lock of the period's
// holiday invoices for the holiday allocation workflow (ADR 0206), before
// the workflow holds Enrollment's period. It reads nothing.
func (h *HolidayContributions) HoldHolidayAllocationInvoices(ctx context.Context, organizationID, accountID, periodID int64) error {
	if organizationID <= 0 || accountID <= 0 {
		return errTraegerRequired
	}
	if err := h.store.LockPeriodBilling(ctx, organizationID, parentbilling.ServiceHoliday, periodID); err != nil {
		return fmt.Errorf("parent billing: hold the invoices of the period: %w", err)
	}
	return nil
}

// RefreezeHolidayAllocationContributions computes the contribution of every
// registration that gained a place, with its sibling group, after the
// workflow's rebooking, capacity or decision (ADR 0206). The workflow still
// holds the billing lock and the period; this takes no further lock.
func (h *HolidayContributions) RefreezeHolidayAllocationContributions(ctx context.Context, organizationID, accountID, periodID int64, registrationIDs []int64) error {
	if organizationID <= 0 || accountID <= 0 {
		return errTraegerRequired
	}
	return h.refreezeMovedUp(ctx, parentbilling.HolidayReviewer{OrganizationID: organizationID, AccountID: accountID}, periodID, registrationIDs)
}

func movedUpOrEmpty(ids []int64) []int64 {
	if ids == nil {
		return []int64{}
	}
	return slices.Compact(slices.Sorted(slices.Values(ids)))
}
