package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (e engine) CountPlannedStudentAssignmentsAfter(ctx context.Context, studentIDs []int64, after string, removals []timetable.InstanceStudent) (map[int64]int, error) {
	value, err := e.service.CountPlannedStudentAssignmentsAfter(ctx, studentIDs, after, domainCareExitAssignments(removals))
	return value, mapError(err)
}

func (e engine) RemovePlannedStudentAssignmentsAfter(ctx context.Context, studentIDs []int64, after string) ([]timetable.InstanceStudent, error) {
	value, err := e.service.RemovePlannedStudentAssignmentsAfter(ctx, studentIDs, after)
	result := make([]timetable.InstanceStudent, 0, len(value))
	for _, row := range value {
		result = append(result, instanceStudentToPublic(row))
	}
	return result, mapError(err)
}

func (e engine) RestoreCareExitStudentAssignments(ctx context.Context, studentIDs, roomIDs, statusDayIDs, pickupExceptionIDs []int64, removals []timetable.InstanceStudent) (int64, error) {
	value, err := e.service.RestoreCareExitStudentAssignments(ctx, studentIDs, roomIDs, statusDayIDs, pickupExceptionIDs, domainCareExitAssignments(removals))
	return value, mapError(err)
}

func domainCareExitAssignments(removals []timetable.InstanceStudent) []domain.InstanceStudent {
	result := make([]domain.InstanceStudent, 0, len(removals))
	for _, row := range removals {
		result = append(result, domain.InstanceStudent(row))
	}
	return result
}
