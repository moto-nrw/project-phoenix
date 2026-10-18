package api

import (
	"context"
	"errors"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"

	"github.com/moto-nrw/project-phoenix/modules/careplan"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// Companion edges are Care Plan records that the retained student services
// still read through the people model contract. This repository translates
// that contract onto the owner's Commands and Queries.

type CompanionRepository struct{ Capability careplan.Capability }

// NewStudentCompanionRepository serves the companion edge contract over the
// Care Plan owner capability.
func NewStudentCompanionRepository(capability careplan.Capability) CompanionRepository {
	return CompanionRepository{Capability: capability}
}

func (r CompanionRepository) ListForStudent(ctx context.Context, studentID int64) ([]*careplan.CompanionEdge, error) {
	values, err := r.Capability.ListCompanionEdges(ctx, studentID)
	if err != nil {
		return nil, WrapRetainedError("list student companions", err)
	}
	result := make([]*careplan.CompanionEdge, 0, len(values))
	for _, value := range values {
		result = append(result, &value)
	}
	return result, nil
}

func (r CompanionRepository) ListLinksForStudents(ctx context.Context, studentIDs []int64) (map[int64][]departure.CompanionLink, error) {
	values, err := r.Capability.ListCompanionLinks(ctx, studentIDs)
	if err != nil {
		return nil, WrapRetainedError("list student companions", err)
	}
	result := make(map[int64][]departure.CompanionLink, len(values))
	for studentID, links := range values {
		converted := make([]departure.CompanionLink, 0, len(links))
		for _, link := range links {
			converted = append(converted, departure.CompanionLink{CompanionStudentID: link.CompanionStudentID, FirstName: link.FirstName, LastName: link.LastName, Weekdays: link.Weekdays})
		}
		result[studentID] = converted
	}
	return result, nil
}

// CompanionIDsForWeekday binds the owner's weekday refusal to the people
// model's sentinel.
func (r CompanionRepository) CompanionIDsForWeekday(ctx context.Context, studentIDs []int64, weekday int) (map[int64][]int64, error) {
	values, err := r.Capability.CompanionIDsForWeekday(ctx, studentIDs, weekday)
	if errors.Is(err, careplan.ErrInvalidCompanion) {
		return nil, departure.ErrCompanionInvalidWeekday
	}
	return values, WrapRetainedError("list companions for weekday", err)
}

func (r CompanionRepository) CompanionCountsExcluding(ctx context.Context, studentIDs []int64, excludeID int64) (map[int64]int, error) {
	values, err := r.Capability.CompanionCountsExcluding(ctx, studentIDs, excludeID)
	return values, WrapRetainedError("count student companions", err)
}

func (r CompanionRepository) CompanionDaysCoveredExcluding(ctx context.Context, studentIDs []int64, excludeID int64) (map[int64]map[string]bool, error) {
	values, err := r.Capability.CompanionDaysCoveredExcluding(ctx, studentIDs, excludeID)
	return values, WrapRetainedError("list student companion days", err)
}

func (r CompanionRepository) CompanionWeekdays(ctx context.Context, studentID int64) ([]int, error) {
	values, err := r.Capability.CompanionWeekdays(ctx, studentID)
	return values, WrapRetainedError("check student companion links", err)
}

func (r CompanionRepository) DeleteEdges(ctx context.Context, edgeIDs []int64) error {
	return WrapRetainedError("delete student companion edges", r.Capability.DeleteCompanionEdges(ctx, edgeIDs))
}

// studentLifecycle maps the owner's lifecycle status onto the three states the
// presence flows branch on. Every other status, today only "pending", is
// StudentLifecycleOther and is treated as not-yet-active, which is what the
// enrollment filter and the check-in guards already did for it.
func studentLifecycle(status string) studentpresence.StudentLifecycle {
	switch status {
	case peopleValues.StudentStatusActive:
		return studentpresence.StudentLifecycleActive
	case peopleValues.StudentStatusInactive:
		return studentpresence.StudentLifecycleInactive
	case peopleValues.StudentStatusAlumnus:
		return studentpresence.StudentLifecycleAlumnus
	case peopleValues.StudentStatusPending:
		return studentpresence.StudentLifecycleOther
	default:
		return studentpresence.StudentLifecycleOther
	}
}

func StudentRecord(row *peopleValues.StudentState) *studentpresence.StudentRecord {
	if row == nil {
		return nil
	}
	return &studentpresence.StudentRecord{
		ID:            row.ID,
		TenantID:      row.TenantID,
		PersonID:      row.PersonID,
		GroupID:       row.GroupID,
		SchoolClass:   row.SchoolClass,
		Lifecycle:     studentLifecycle(row.Status),
		EnrolledFrom:  row.EnrolledFrom,
		EnrolledUntil: row.EnrolledUntil,
		Sick:          row.Sick,
		SickSince:     row.SickSince,
		Excused:       row.Excused,
		ExcusedSince:  row.ExcusedSince,
	}
}
