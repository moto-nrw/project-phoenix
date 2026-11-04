package application

import (
	"context"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// A cancellation in a sibling group (#5662, ADR 0115 addendum): the
// cancelled registration leaves the group at once, and the rest of the
// group gets its frozen contributions computed again, in the transaction of
// the cancellation. Nothing waits for the next merge, split or confirmation.

// CancelHolidayRegistration holds the invoices of the period and the period
// in Enrollment like a merge, then reads the group, cancels the
// registration through Enrollment, takes it out of its group and freezes
// the contribution of every other confirmed registration of the group
// again, also one an issued invoice bills: the invoice keeps its own
// positions, and the page of the registration shows that it billed another
// amount. A registration whose frozen exemption no rule allows any more
// keeps its contribution, so no rule of another registration stops the
// cancellation. Every registration of the
// group records the change in its audit trail. The caller rolls the
// transaction back when this fails after the cancellation.
func (h *HolidayContributions) CancelHolidayRegistration(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID, registrationID int64) (parentbilling.HolidayCancellation, error) {
	if reviewer.OrganizationID <= 0 || reviewer.AccountID <= 0 {
		return parentbilling.HolidayCancellation{}, errTraegerRequired
	}
	groups, _, err := h.lockSiblings(ctx, reviewer, periodID)
	if err != nil {
		return parentbilling.HolidayCancellation{}, err
	}
	registration, err := h.registrations.CancelHolidayRegistration(ctx, reviewer, periodID, registrationID)
	if err != nil {
		return parentbilling.HolidayCancellation{}, err
	}
	cancelled := parentbilling.HolidayCancellation{RegistrationID: registration.ID, Status: registration.Status}
	group, grouped := groupOf(groups, registration.ID)
	if !grouped {
		return cancelled, nil
	}
	ids := slices.DeleteFunc(slices.Clone(group.RegistrationIDs), func(id int64) bool { return id == registration.ID })
	rest, err := h.readSiblings(ctx, reviewer, periodID, ids, nil)
	if err != nil {
		return parentbilling.HolidayCancellation{}, err
	}
	if err := h.store.RemoveHolidaySibling(ctx, reviewer.OrganizationID, registration.ID); err != nil {
		return parentbilling.HolidayCancellation{}, fmt.Errorf("parent billing: take the cancelled registration out of its group: %w", err)
	}
	// An issued invoice keeps its own positions, so a registration it bills
	// gets its new contribution too: the next run bills it once the
	// Träger-Büro cancels the invoice. Its page shows the difference.
	if err := h.refreeze(ctx, reviewer, periodID, rest, nil, true); err != nil {
		return parentbilling.HolidayCancellation{}, err
	}
	left := []siblingMember{{registration: registration}}
	if err := h.recordSiblings(ctx, reviewer, periodID, left, parentbilling.HolidaySiblingsCancelled, ids...); err != nil {
		return parentbilling.HolidayCancellation{}, err
	}
	if err := h.recordSiblings(ctx, reviewer, periodID, rest, parentbilling.HolidaySiblingsCancelled, registration.ID); err != nil {
		return parentbilling.HolidayCancellation{}, err
	}
	return cancelled, nil
}
