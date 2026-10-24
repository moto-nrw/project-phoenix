package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

func (s *Service) DeleteRetiredStaffAssignments(ctx context.Context, staffID int64) (timetable.RetiredStaffAssignments, error) {
	if staffID <= 0 {
		return timetable.RetiredStaffAssignments{}, s.reject("delete_retired_staff_assignments", timetable.ErrInvalidInstanceStaff)
	}
	plannedSupervisors, err := s.DeletePlannedSupervisorsByStaff(ctx, staffID)
	if err != nil {
		return timetable.RetiredStaffAssignments{}, err
	}
	instanceStaff, err := s.deleteInstanceStaffByStaff(ctx, staffID)
	if err != nil {
		return timetable.RetiredStaffAssignments{}, err
	}
	return timetable.RetiredStaffAssignments{PlannedSupervisors: plannedSupervisors, InstanceStaff: instanceStaff}, nil
}
