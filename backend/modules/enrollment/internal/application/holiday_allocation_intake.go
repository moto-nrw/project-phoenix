package application

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
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
// missing one is a validation error on criteria.<id> or
// children.<n>.criteria.<id>, so the form marks the question. A period by
// arrival keeps no answer.
func (h *HolidayRegistrations) checkCriteria(ctx context.Context, organizationID int64, period enrollment.HolidayPeriod, input enrollment.HolidayRegistrationInput) (enrollment.HolidayRegistrationInput, error) {
	questions, err := h.criterionQuestions(ctx, organizationID, period)
	if err != nil {
		return input, err
	}
	missing := validation.Errors{}
	input.Criteria = keepHolidayCriteria(questions, enrollment.HolidayCriterionFamily, input.Criteria, "criteria.", missing)
	children := make([]enrollment.HolidayChildInput, len(input.Children))
	for i, child := range input.Children {
		child.Criteria = keepHolidayCriteria(questions, enrollment.HolidayCriterionChild, child.Criteria, fmt.Sprintf("children.%d.criteria.", i), missing)
		children[i] = child
	}
	input.Children = children
	if len(missing) > 0 {
		return input, missing
	}
	return input, nil
}

// keepHolidayCriteria keeps the answers to the questions of scope and names
// every unanswered one in missing under prefix and its ID.
func keepHolidayCriteria(questions []enrollment.HolidayCriterionQuestion, scope string, given map[int64]bool, prefix string, missing validation.Errors) map[int64]bool {
	kept := map[int64]bool{}
	for _, question := range questions {
		if question.Scope != scope {
			continue
		}
		value, answered := given[question.ID]
		if !answered {
			missing[prefix+strconv.FormatInt(question.ID, 10)] = errors.New("answer yes or no")
			continue
		}
		kept[question.ID] = value
	}
	return kept
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
