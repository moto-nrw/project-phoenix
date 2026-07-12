package activities

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/activities"
)

// UpdateGroupWithDetails updates a group's core fields, replaces its
// supervisor set, and replaces its schedules as one unit. Every failure
// aborts the whole update and propagates — callers run this inside a tenant
// transaction, so a supervisor or schedule failure rolls back the group
// update too.
//
// Behavior change (issue #575 B10, deliberate): the historical handler
// orchestration logged supervisor/schedule failures at Warn and COMMITTED
// the partial state, returning HTTP 200 — a rename could land while the
// schedule replacement half-failed (old rows deleted, new rows missing).
// Clients now get the error and unchanged state instead of a silent
// partial write.
func (s *Service) UpdateGroupWithDetails(ctx context.Context, group *activities.Group, requestingStaffID int64, hasManagePermission bool, supervisorIDs []int64, schedules []*activities.Schedule) (*activities.Group, error) {
	updatedGroup, err := s.UpdateGroup(ctx, group, requestingStaffID, hasManagePermission)
	if err != nil {
		return nil, err
	}

	if err := s.UpdateGroupSupervisors(ctx, updatedGroup.ID, supervisorIDs); err != nil {
		return nil, err
	}

	existingSchedules, err := s.GetGroupSchedules(ctx, updatedGroup.ID)
	if err != nil {
		return nil, err
	}
	for _, schedule := range existingSchedules {
		if err := s.DeleteSchedule(ctx, schedule.ID); err != nil {
			return nil, err
		}
	}
	for _, schedule := range schedules {
		if _, err := s.AddSchedule(ctx, updatedGroup.ID, schedule); err != nil {
			return nil, err
		}
	}

	return updatedGroup, nil
}
