package repositories

import (
	"context"
	"slices"

	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// personChildRepository attaches the children's names and applies the
// deleted-person filter the cross-tenant join used to carry.
type personChildRepository struct {
	parentModels.ChildRepository
	persons peopledirectory.Query
}

func (r personChildRepository) ListByAccount(ctx context.Context, accountID int64) ([]*parentModels.ChildSummary, error) {
	children, err := r.ChildRepository.ListByAccount(ctx, accountID)
	if err != nil || len(children) == 0 {
		return children, err
	}
	kept, err := attachChildPersons(ctx, r.persons, children)
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(kept, func(left, right *parentModels.ChildSummary) int {
		if order := compareStrings(left.FirstName, right.FirstName); order != 0 {
			return order
		}
		return compareStrings(left.LastName, right.LastName)
	})
	return kept, nil
}

func (r personChildRepository) FindForAccount(ctx context.Context, accountID, studentID int64) (*parentModels.ChildSummary, error) {
	child, err := r.ChildRepository.FindForAccount(ctx, accountID, studentID)
	if err != nil || child == nil {
		return child, err
	}
	kept, err := attachChildPersons(ctx, r.persons, []*parentModels.ChildSummary{child})
	if err != nil {
		return nil, err
	}
	if len(kept) == 0 {
		// A soft-deleted person hides the child, as the join filter did.
		return nil, nil
	}
	return kept[0], nil
}

func attachChildPersons(ctx context.Context, query peopledirectory.Query, children []*parentModels.ChildSummary) ([]*parentModels.ChildSummary, error) {
	ids := make([]int64, 0, len(children))
	for _, child := range children {
		ids = append(ids, child.PersonID)
	}
	persons, err := personsByID(ctx, query, ids)
	if err != nil {
		return nil, err
	}
	kept := make([]*parentModels.ChildSummary, 0, len(children))
	for _, child := range children {
		person, found := persons[child.PersonID]
		if !found {
			continue
		}
		child.FirstName = person.FirstName
		child.LastName = person.LastName
		kept = append(kept, child)
	}
	return kept, nil
}

// personEnrollablePhaseRepository decides the existing_students eligibility:
// the phase stays only when at least one candidate person still exists (a
// soft-deleted child does not count), mirroring the submit-time matcher.
type personEnrollablePhaseRepository struct {
	parentModels.EnrollablePhaseRepository
	persons peopledirectory.Query
}

func (r personEnrollablePhaseRepository) ListEnrollable(ctx context.Context, accountID int64) ([]*parentModels.EnrollablePhase, error) {
	phases, err := r.EnrollablePhaseRepository.ListEnrollable(ctx, accountID)
	if err != nil || len(phases) == 0 {
		return phases, err
	}
	ids := make([]int64, 0, len(phases))
	for _, phase := range phases {
		ids = append(ids, phase.EnrolledSubmitPersonIDs...)
	}
	persons, err := personsByID(ctx, r.persons, ids)
	if err != nil {
		return nil, err
	}
	result := make([]*parentModels.EnrollablePhase, 0, len(phases))
	for _, phase := range phases {
		if phase.Audience == "existing_students" && !anyPersonExists(persons, phase.EnrolledSubmitPersonIDs) {
			continue
		}
		result = append(result, phase)
	}
	return result, nil
}

func (r personEnrollablePhaseRepository) GuardianSubmitStatus(ctx context.Context, accountID, tenantID int64) (*parentModels.GuardianSubmitStatus, error) {
	status, err := r.EnrollablePhaseRepository.GuardianSubmitStatus(ctx, accountID, tenantID)
	if err != nil || status == nil {
		return status, err
	}
	persons, err := personsByID(ctx, r.persons, status.EnrolledSubmitPersonIDs)
	if err != nil {
		return nil, err
	}
	status.HasEnrolledSubmitPermission = anyPersonExists(persons, status.EnrolledSubmitPersonIDs)
	return status, nil
}

func anyPersonExists(persons map[int64]peopledirectory.Person, ids []int64) bool {
	for _, id := range ids {
		if _, found := persons[id]; found {
			return true
		}
	}
	return false
}
