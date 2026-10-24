package application

import (
	"context"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	parentModels "github.com/moto-nrw/project-phoenix/modules/careplan/internal/rows"
)

// WithParentChildPersons attaches the owner's person projection and its
// soft-delete filter to the retained child reads.
func WithParentChildPersons(children parentModels.ChildRepository, directory careplan.ParentPortalDirectory) parentModels.ChildRepository {
	return personChildRepository{ChildRepository: children, persons: directory}
}

// WithParentPhasePersons applies the existing-student audience facts.
func WithParentPhasePersons(phases parentModels.EnrollablePhaseRepository, directory careplan.ParentPortalDirectory) parentModels.EnrollablePhaseRepository {
	return personEnrollablePhaseRepository{EnrollablePhaseRepository: phases, persons: directory}
}

// parentPortalPersonsByID resolves the persons the directory still shows for
// ids, keyed by person ID. Duplicates and unset IDs are dropped before the
// lookup.
func parentPortalPersonsByID(ctx context.Context, directory careplan.ParentPortalDirectory, ids []int64) (map[int64]careplan.ParentPortalPerson, error) {
	unique := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, found := seen[id]; !found {
			seen[id] = struct{}{}
			unique = append(unique, id)
		}
	}
	if len(unique) == 0 {
		return map[int64]careplan.ParentPortalPerson{}, nil
	}
	values, err := directory.ListParentPortalPersons(ctx, unique)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]careplan.ParentPortalPerson, len(values))
	for _, value := range values {
		result[value.ID] = value
	}
	return result, nil
}

// personChildRepository attaches the children's names and applies the
// deleted-person filter the cross-tenant join used to carry.
type personChildRepository struct {
	parentModels.ChildRepository
	persons careplan.ParentPortalDirectory
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
		if order := strings.Compare(left.FirstName, right.FirstName); order != 0 {
			return order
		}
		return strings.Compare(left.LastName, right.LastName)
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

func attachChildPersons(ctx context.Context, directory careplan.ParentPortalDirectory, children []*parentModels.ChildSummary) ([]*parentModels.ChildSummary, error) {
	ids := make([]int64, 0, len(children))
	for _, child := range children {
		ids = append(ids, child.PersonID)
	}
	persons, err := parentPortalPersonsByID(ctx, directory, ids)
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
	persons careplan.ParentPortalDirectory
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
	persons, err := parentPortalPersonsByID(ctx, r.persons, ids)
	if err != nil {
		return nil, err
	}
	result := make([]*parentModels.EnrollablePhase, 0, len(phases))
	for _, phase := range phases {
		if phase.Audience == "existing_students" && !anyParentPortalPersonExists(persons, phase.EnrolledSubmitPersonIDs) {
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
	persons, err := parentPortalPersonsByID(ctx, r.persons, status.EnrolledSubmitPersonIDs)
	if err != nil {
		return nil, err
	}
	status.HasEnrolledSubmitPermission = anyParentPortalPersonExists(persons, status.EnrolledSubmitPersonIDs)
	return status, nil
}

func anyParentPortalPersonExists(persons map[int64]careplan.ParentPortalPerson, ids []int64) bool {
	for _, id := range ids {
		if _, found := persons[id]; found {
			return true
		}
	}
	return false
}
