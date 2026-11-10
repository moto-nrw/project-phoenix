package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// The office's half of the holiday care wish (#4703): a child of a school
// of the Träger whose wish is unknown carries the mark "check" in a period
// that takes the wish into account, and the Träger-Büro decides yes or no.
// The decision only orders the waiting list; it gives and takes no place.

// HolidayWishReview serves enrollment.HolidayWishReview.
type HolidayWishReview struct {
	review *HolidayRegistrationReview
	wishes HolidayWishDecisionStore
}

var _ enrollment.HolidayWishReview = (*HolidayWishReview)(nil)

// NewHolidayWishReview builds the office's decision on the wish over the
// review, which locks and reads the registration.
func NewHolidayWishReview(review *HolidayRegistrationReview, wishes HolidayWishDecisionStore) *HolidayWishReview {
	if review == nil || wishes == nil {
		panic("enrollment: the holiday wish review needs the review and the wish store")
	}
	return &HolidayWishReview{review: review, wishes: wishes}
}

// SetHolidayWish locks the period and the registration, writes the office's
// decision on the child's wish and records it in the registration's trail.
// A cancelled registration is a state conflict; a holiday guest or a value
// other than yes and no is ErrHolidayWishInvalid.
func (w *HolidayWishReview) SetHolidayWish(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID, registrationID, childID int64, wish string) (enrollment.HolidayRegistrationDetail, error) {
	if wish != enrollment.HolidayWishYes && wish != enrollment.HolidayWishNo {
		return enrollment.HolidayRegistrationDetail{}, enrollment.ErrHolidayWishInvalid
	}
	if childID <= 0 {
		return enrollment.HolidayRegistrationDetail{}, enrollment.ErrHolidayRegistrationEntryNotFound
	}
	return w.review.change(ctx, reviewer, periodID, registrationID, func(period enrollment.HolidayPeriod, current holidayRegistrationState) (string, any, error) {
		if current.Status == enrollment.HolidayRegistrationCancelled {
			return "", nil, enrollment.ErrHolidayRegistrationStateConflict
		}
		read, _, err := w.review.store.ReadHolidayReview(ctx, reviewer.OrganizationID, period.ID, registrationID)
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
		written, err := w.wishes.SetHolidayChildWish(ctx, reviewer.OrganizationID, registrationID, childID, wish, enrollment.HolidayWishSourceOffice)
		if err != nil {
			return "", nil, fmt.Errorf("enrollment: write holiday wish: %w", err)
		}
		if !written {
			return "", nil, enrollment.ErrHolidayRegistrationEntryNotFound
		}
		return enrollment.HolidayAuditWish, enrollment.HolidayRegistrationEventDetails{ChildID: childID, From: child.HolidayWish, Wish: wish}, nil
	})
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
