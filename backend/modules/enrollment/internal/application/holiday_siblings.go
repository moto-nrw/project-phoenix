package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// Siblings from separate holiday registrations (#5628): the Elternabrechnung
// keeps the sibling groups; Enrollment records a merge or split in the audit
// trail of each registration it concerns.

// HoldHolidayPeriod locks the period like the start of every change of a
// registration (#5628): a cancellation or rebooking of the period waits
// until the caller's transaction ends.
func (h *HolidayRegistrationReview) HoldHolidayPeriod(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID int64) error {
	if err := holidayReviewKey(reviewer, periodID, 1); err != nil {
		return err
	}
	_, err := h.period(ctx, reviewer.OrganizationID, periodID, true)
	return err
}

// RecordHolidaySiblings records a sibling merge or split of the
// Elternabrechnung (#5628) in the audit trail of the registration, under
// the lock of the period and the registration like every change.
func (h *HolidayRegistrationReview) RecordHolidaySiblings(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID, registrationID int64, action string, siblingIDs []int64) error {
	if action != enrollment.HolidayAuditSiblingMerge && action != enrollment.HolidayAuditSiblingSplit {
		return fmt.Errorf("enrollment: unknown sibling action %q", action)
	}
	_, err := h.change(ctx, reviewer, periodID, registrationID, func(enrollment.HolidayPeriod, holidayRegistrationState) (string, any, error) {
		return action, enrollment.HolidayRegistrationEventDetails{Siblings: siblingIDs}, nil
	})
	return err
}
