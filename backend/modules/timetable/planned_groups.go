package timetable

import "context"

// ListPlannedGroupsForStaff returns the activity groups the staff member has
// any planned supervision for, ordered by ID. The validity window of the
// planned row is not applied, matching the retained "my activity groups" read.
func (m *Module) ListPlannedGroupsForStaff(ctx context.Context, staffID int64) ([]Group, error) {
	if staffID <= 0 {
		return nil, m.reject("list_planned_groups_for_staff", ErrInvalidPlannedSupervisorQuery)
	}
	supervisors, err := m.engine.ListPlannedSupervisors(ctx, PlannedSupervisorFilter{StaffID: &staffID})
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
		return []Group{}, nil
	}
	return m.engine.ListGroups(ctx, GroupFilter{IDs: groupIDs, OrderByID: true})
}
