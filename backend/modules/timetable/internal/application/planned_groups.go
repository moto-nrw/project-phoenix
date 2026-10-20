package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// ListPlannedGroupsForStaff returns the activity groups the staff member has
// any planned supervision for, ordered by ID. The validity window of the
// planned row is not applied, matching the retained "my activity groups" read.
func (s *Service) ListPlannedGroupsForStaff(ctx context.Context, staffID int64) ([]timetable.Group, error) {
	if staffID <= 0 {
		return nil, s.reject("list_planned_groups_for_staff", timetable.ErrInvalidPlannedSupervisorQuery)
	}
	supervisors, err := s.ListPlannedSupervisors(ctx, timetable.PlannedSupervisorFilter{StaffID: &staffID})
	if err != nil {
		return nil, err
	}
	seen := make(map[int64]struct{}, len(supervisors))
	groupIDs := make([]int64, 0, len(supervisors))
	for _, supervisor := range supervisors {
		if _, ok := seen[supervisor.GroupID]; ok {
			continue
		}
		seen[supervisor.GroupID] = struct{}{}
		groupIDs = append(groupIDs, supervisor.GroupID)
	}
	if len(groupIDs) == 0 {
		return []timetable.Group{}, nil
	}
	return s.ListGroups(ctx, timetable.GroupFilter{IDs: groupIDs, OrderByID: true})
}
