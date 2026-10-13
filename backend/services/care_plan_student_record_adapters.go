package services

import (
	"context"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// Companion edges are Care Plan records that the retained student services
// still read through the people model contract. This repository translates
// that contract onto the owner's Commands and Queries.

type companionRepository struct{ capability careplan.Capability }

var _ userModels.StudentCompanionRepository = companionRepository{}

// NewStudentCompanionRepository serves the companion edge contract over the
// Care Plan owner capability.
func NewStudentCompanionRepository(capability careplan.Capability) userModels.StudentCompanionRepository {
	return companionRepository{capability: capability}
}

func (r companionRepository) ListForStudent(ctx context.Context, studentID int64) ([]*userModels.StudentCompanion, error) {
	values, err := r.capability.ListCompanionEdges(ctx, studentID)
	if err != nil {
		return nil, wrapRetainedError("list student companions", err)
	}
	result := make([]*userModels.StudentCompanion, 0, len(values))
	for _, value := range values {
		result = append(result, companionToLegacy(value))
	}
	return result, nil
}

func (r companionRepository) ListLinksForStudents(ctx context.Context, studentIDs []int64) (map[int64][]userModels.CompanionLink, error) {
	values, err := r.capability.ListCompanionLinks(ctx, studentIDs)
	if err != nil {
		return nil, wrapRetainedError("list student companions", err)
	}
	result := make(map[int64][]userModels.CompanionLink, len(values))
	for studentID, links := range values {
		converted := make([]userModels.CompanionLink, 0, len(links))
		for _, link := range links {
			converted = append(converted, userModels.CompanionLink{CompanionStudentID: link.CompanionStudentID, FirstName: link.FirstName, LastName: link.LastName, Weekdays: link.Weekdays})
		}
		result[studentID] = converted
	}
	return result, nil
}

func (r companionRepository) CompanionIDsForWeekday(ctx context.Context, studentIDs []int64, weekday int) (map[int64][]int64, error) {
	if _, ok := userModels.CompanionWeekdayKeys[weekday]; !ok {
		return nil, userModels.ErrCompanionInvalidWeekday
	}
	values, err := r.capability.CompanionIDsForWeekday(ctx, studentIDs, weekday)
	return values, wrapRetainedError("list companions for weekday", err)
}

func (r companionRepository) CompanionCountsExcluding(ctx context.Context, studentIDs []int64, excludeID int64) (map[int64]int, error) {
	values, err := r.capability.CompanionCountsExcluding(ctx, studentIDs, excludeID)
	return values, wrapRetainedError("count student companions", err)
}

func (r companionRepository) CompanionDaysCoveredExcluding(ctx context.Context, studentIDs []int64, excludeID int64) (map[int64]map[string]bool, error) {
	values, err := r.capability.CompanionDaysCoveredExcluding(ctx, studentIDs, excludeID)
	return values, wrapRetainedError("list student companion days", err)
}

func (r companionRepository) CompanionWeekdays(ctx context.Context, studentID int64) ([]int, error) {
	values, err := r.capability.CompanionWeekdays(ctx, studentID)
	return values, wrapRetainedError("check student companion links", err)
}

func (r companionRepository) DeleteEdges(ctx context.Context, edgeIDs []int64) error {
	return wrapRetainedError("delete student companion edges", r.capability.DeleteCompanionEdges(ctx, edgeIDs))
}

func companionToLegacy(value careplan.CompanionEdge) *userModels.StudentCompanion {
	result := &userModels.StudentCompanion{StudentLowID: value.StudentLowID, StudentHighID: value.StudentHighID, Weekday: value.Weekday}
	result.ID, result.TenantID, result.CreatedAt, result.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	return result
}
