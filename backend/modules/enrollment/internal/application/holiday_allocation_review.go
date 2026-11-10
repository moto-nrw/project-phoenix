package application

import (
	"context"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// The allocation by criteria in the review of a registration (#4700): a new
// booking of a received registration waits for the Büro, a registration
// with such a booking is not confirmed, and the detail names the answers.

// holidayWaitingPlacement is the placement a new or grown booking takes: in
// a period by criteria a received registration's booking waits for the
// decision of the Büro; every other one waits on the waiting list, from
// where the placement of #4699 moves it. A confirmed registration has no
// pending booking, so its contribution never waits on a decision.
func holidayWaitingPlacement(period enrollment.HolidayPeriod, status string) string {
	if period.AllocationMode == enrollment.HolidayAllocationCriteria && status == enrollment.HolidayRegistrationReceived {
		return enrollment.HolidayPlacementPending
	}
	return enrollment.HolidayPlacementWaitlisted
}

// refusePendingAllocation refuses to confirm a registration of a period by
// criteria with a booking the Büro has not decided on.
func (h *HolidayRegistrationReview) refusePendingAllocation(ctx context.Context, organizationID int64, period enrollment.HolidayPeriod, registrationID int64) error {
	if period.AllocationMode != enrollment.HolidayAllocationCriteria {
		return nil
	}
	read, _, err := h.store.ReadHolidayReview(ctx, organizationID, period.ID, registrationID)
	if err != nil {
		return fmt.Errorf("enrollment: read holiday registration: %w", err)
	}
	for _, registration := range read {
		for _, child := range registration.Children {
			if slices.ContainsFunc(child.Bookings, func(booking enrollment.HolidayRegistrationBooking) bool {
				return booking.Placement == enrollment.HolidayPlacementPending
			}) {
				return enrollment.ErrHolidayAllocationPending
			}
		}
	}
	return nil
}

// detailCriteria names the answers of a registration to the active
// criteria of a period by criteria: the family's once, a child criterion
// per child; an answer the parents never gave is unknown. A period by
// arrival names none.
func (h *HolidayRegistrationReview) detailCriteria(ctx context.Context, organizationID int64, period enrollment.HolidayPeriod, detail enrollment.HolidayRegistrationDetail) ([]enrollment.HolidayRegistrationCriterion, error) {
	criteria := []enrollment.HolidayRegistrationCriterion{}
	if period.AllocationMode != enrollment.HolidayAllocationCriteria {
		return criteria, nil
	}
	all, err := h.store.ReadHolidayCriteria(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("enrollment: read holiday criteria: %w", err)
	}
	answers, err := h.store.ReadHolidayCriterionAnswers(ctx, organizationID, period.ID, detail.ID)
	if err != nil {
		return nil, fmt.Errorf("enrollment: read holiday criterion answers: %w", err)
	}
	find := func(criterionID int64, childID *int64) (enrollment.HolidayCriterionAnswer, bool) {
		index := slices.IndexFunc(answers, func(answer enrollment.HolidayCriterionAnswer) bool {
			return answer.CriterionID == criterionID && ((answer.ChildID == nil && childID == nil) ||
				(answer.ChildID != nil && childID != nil && *answer.ChildID == *childID))
		})
		if index < 0 {
			return enrollment.HolidayCriterionAnswer{}, false
		}
		return answers[index], true
	}
	add := func(criterion enrollment.HolidayCriterion, childID *int64) {
		entry := enrollment.HolidayRegistrationCriterion{
			CriterionID: criterion.ID, Name: criterion.Name, Question: criterion.Question, Scope: criterion.Scope,
			ChildID: childID, Stated: enrollment.HolidayCriterionUnknown,
		}
		if answer, found := find(criterion.ID, childID); found {
			if answer.Stated != nil {
				entry.Stated = holidayYesNo(*answer.Stated)
			}
			if answer.Checked != nil {
				checked := holidayYesNo(*answer.Checked)
				entry.Checked, entry.CheckedAt = &checked, answer.CheckedAt
			}
		}
		criteria = append(criteria, entry)
	}
	for _, criterion := range activeHolidayCriteria(all) {
		if criterion.Scope == enrollment.HolidayCriterionFamily {
			add(criterion, nil)
			continue
		}
		for _, child := range detail.Children {
			add(criterion, &child.ID)
		}
	}
	return criteria, nil
}
