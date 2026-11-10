package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// The allocation by criteria in the public registration (#4700): the form
// asks the active criteria of the Träger, family criteria once and child
// criteria for each child, and the bookings wait for the decision of the
// Büro. Only the answers the active criteria need are kept.

// HolidayCriteriaReader reads the criteria of a Träger in its Träger
// transaction.
type HolidayCriteriaReader interface {
	ReadHolidayCriteria(ctx context.Context, organizationID int64) ([]enrollment.HolidayCriterion, error)
}

// criterionQuestions are the questions of the active criteria a period by
// criteria asks, in their priority; a period by arrival asks none.
func (h *HolidayRegistrations) criterionQuestions(ctx context.Context, organizationID int64, period enrollment.HolidayPeriod) ([]enrollment.HolidayCriterionQuestion, error) {
	questions := []enrollment.HolidayCriterionQuestion{}
	if period.AllocationMode != enrollment.HolidayAllocationCriteria {
		return questions, nil
	}
	criteria, err := h.deps.Criteria.ReadHolidayCriteria(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("enrollment: read holiday criteria: %w", err)
	}
	for _, criterion := range activeHolidayCriteria(criteria) {
		questions = append(questions, enrollment.HolidayCriterionQuestion{ID: criterion.ID, Question: criterion.Question, Scope: criterion.Scope})
	}
	return questions, nil
}

// checkCriteria keeps of the answers exactly those the active criteria of a
// period by criteria need: the family's to the family criteria and each
// child's to the child criteria. An answer to anything else is dropped; a
// missing one means the parents filled in an older form, which they are
// told. A period by arrival keeps no answer.
func (h *HolidayRegistrations) checkCriteria(ctx context.Context, organizationID int64, period enrollment.HolidayPeriod, input enrollment.HolidayRegistrationInput) (enrollment.HolidayRegistrationInput, error) {
	questions, err := h.criterionQuestions(ctx, organizationID, period)
	if err != nil {
		return input, err
	}
	keep := func(given map[int64]bool, scope string) (map[int64]bool, bool) {
		kept := map[int64]bool{}
		for _, question := range questions {
			if question.Scope != scope {
				continue
			}
			value, answered := given[question.ID]
			if !answered {
				return nil, false
			}
			kept[question.ID] = value
		}
		return kept, true
	}
	family, complete := keep(input.Criteria, enrollment.HolidayCriterionFamily)
	if !complete {
		return input, enrollment.ErrHolidayRegistrationFormChanged
	}
	input.Criteria = family
	children := make([]enrollment.HolidayChildInput, len(input.Children))
	for i, child := range input.Children {
		if child.Criteria, complete = keep(child.Criteria, enrollment.HolidayCriterionChild); !complete {
			return input, enrollment.ErrHolidayRegistrationFormChanged
		}
		children[i] = child
	}
	input.Children = children
	return input, nil
}

// pendingHolidayPlacements names every booking of a registration of a
// period by criteria as pending: no place is given when the form is sent.
func pendingHolidayPlacements(input enrollment.HolidayRegistrationInput) []enrollment.HolidayReceiptPlacement {
	placements := []enrollment.HolidayReceiptPlacement{}
	for i, child := range input.Children {
		for _, booking := range child.Bookings {
			placements = append(placements, enrollment.HolidayReceiptPlacement{ChildPosition: i + 1, WeekID: booking.WeekID, Placement: enrollment.HolidayPlacementPending})
		}
	}
	return placements
}
