package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func domainCareExitAssignments(removals []timetable.InstanceStudent) []domain.InstanceStudent {
	result := make([]domain.InstanceStudent, 0, len(removals))
	for _, row := range removals {
		result = append(result, domain.InstanceStudent(row))
	}
	return result
}
