package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

func (e Engine) DeleteRetiredStaffAssignments(ctx context.Context, staffID int64) (timetable.RetiredStaffAssignments, error) {
	plannedSupervisors, err := e.service.DeletePlannedSupervisorsByStaff(ctx, staffID)
	if err != nil {
		return timetable.RetiredStaffAssignments{}, MapError(err)
	}
	instanceStaff, err := e.service.deleteInstanceStaffByStaff(ctx, staffID)
	if err != nil {
		return timetable.RetiredStaffAssignments{}, MapError(err)
	}
	return timetable.RetiredStaffAssignments{PlannedSupervisors: plannedSupervisors, InstanceStaff: instanceStaff}, nil
}
