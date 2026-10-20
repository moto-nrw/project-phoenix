package application

import (
	timetable "github.com/moto-nrw/project-phoenix/modules/timetable"
	domain "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func OffboardingCountsToPublic(value domain.OffboardingCounts) timetable.OffboardingCounts {
	return timetable.OffboardingCounts{InstanceAssignments: value.InstanceAssignments, PlannedSupervisors: value.PlannedSupervisors}
}
