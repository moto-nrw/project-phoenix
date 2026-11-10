package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// The office's half of the holiday care wish (#4703): a child of a school
// of the Träger whose wish is unknown carries the mark "check" in a period
// that takes the wish into account, and the Träger-Büro decides yes or no.
// The decision only orders the waiting list; it gives and takes no place.

// errHolidayWishStoreRequired reports a review built without the wish store.
var errHolidayWishStoreRequired = errors.New("enrollment: the holiday wish decision needs the wish store")

// NewHolidayWishReview gives the review the office's decision on the wish
// over wishes and answers it as enrollment.HolidayWishReview.
func NewHolidayWishReview(review *HolidayRegistrationReview, wishes HolidayWishDecisionStore) enrollment.HolidayWishReview {
	if review == nil || wishes == nil {
		panic("enrollment: the holiday wish review needs the review and the wish store")
	}
	review.wishes = wishes
	return review
}

// SetHolidayWish locks the period and the registration, writes the office's
// decision on the child's wish and records it in the registration's trail.
// A cancelled registration is a state conflict; a holiday guest or a value
// other than yes and no is ErrHolidayWishInvalid.
func (h *HolidayRegistrationReview) SetHolidayWish(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID, registrationID, childID int64, wish string) (enrollment.HolidayRegistrationDetail, error) {
	if wish != enrollment.HolidayWishYes && wish != enrollment.HolidayWishNo {
		return enrollment.HolidayRegistrationDetail{}, enrollment.ErrHolidayWishInvalid
	}
	if childID <= 0 {
		return enrollment.HolidayRegistrationDetail{}, enrollment.ErrHolidayRegistrationEntryNotFound
	}
	if h.wishes == nil {
		return enrollment.HolidayRegistrationDetail{}, errHolidayWishStoreRequired
	}
	return h.change(ctx, reviewer, periodID, registrationID, func(period enrollment.HolidayPeriod, current holidayRegistrationState) (string, any, error) {
		if current.Status == enrollment.HolidayRegistrationCancelled {
			return "", nil, enrollment.ErrHolidayRegistrationStateConflict
		}
		return h.decideWish(ctx, reviewer.OrganizationID, period.ID, registrationID, childID, wish)
	})
}

// decide writes the wish of a child of a school of the Träger in the locked
// registration and names the trail entry; an unchanged decision records
// nothing.
func (h *HolidayRegistrationReview) decideWish(ctx context.Context, organizationID, periodID, registrationID, childID int64, wish string) (string, any, error) {
	read, _, err := h.store.ReadHolidayReview(ctx, organizationID, periodID, registrationID)
	if err != nil {
		return "", nil, fmt.Errorf("enrollment: read holiday registration: %w", err)
	}
	child, found := holidayChild(read, childID)
	if !found {
		return "", nil, enrollment.ErrHolidayRegistrationEntryNotFound
	}
	if child.HomeSchoolID == nil {
		return "", nil, enrollment.ErrHolidayWishInvalid
	}
	if child.HolidayWish == wish && child.HolidayWishSource == enrollment.HolidayWishSourceOffice {
		return "", nil, nil
	}
	written, err := h.wishes.SetHolidayChildWish(ctx, organizationID, registrationID, childID, wish, enrollment.HolidayWishSourceOffice)
	if err != nil {
		return "", nil, fmt.Errorf("enrollment: write holiday wish: %w", err)
	}
	if !written {
		return "", nil, enrollment.ErrHolidayRegistrationEntryNotFound
	}
	return enrollment.HolidayAuditWish, enrollment.HolidayRegistrationEventDetails{ChildID: childID, From: child.HolidayWish, Wish: wish}, nil
}

// holidayWishCheck marks a child whose wish the office still has to check:
// the period takes the wish into account, the child has a school of the
// Träger, its wish is unknown and the registration is not cancelled.
func holidayWishCheck(period enrollment.HolidayPeriod, child enrollment.HolidayRegistrationChild, cancelled bool) bool {
	return period.HolidayWishRule && !cancelled && child.HomeSchoolID != nil && child.HolidayWish == ""
}

// validateHolidayWishFilter admits check, yes and no.
func validateHolidayWishFilter(wish string) error {
	switch wish {
	case "", enrollment.HolidayWishFilterCheck, enrollment.HolidayWishFilterYes, enrollment.HolidayWishFilterNo:
		return nil
	default:
		return fmt.Errorf("%w: unknown wish %q", enrollment.ErrInvalidHolidayRegistrationFilter, wish)
	}
}

// holidayWishMatches admits a child of the wish filter.
func holidayWishMatches(child enrollment.HolidayRegistrationChild, wish string) bool {
	switch wish {
	case "":
		return true
	case enrollment.HolidayWishFilterCheck:
		return child.HolidayWishCheck
	default:
		return child.HomeSchoolID != nil && child.HolidayWish == wish
	}
}
