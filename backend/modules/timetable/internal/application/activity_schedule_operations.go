package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
)

// ======== Schedule Methods ========

// AddSchedule adds a schedule to an activity group
func (s *ActivityService) AddSchedule(ctx context.Context, groupID int64, schedule *activityContract.Schedule) (*activityContract.Schedule, error) {
	group, err := s.findMutableActivityGroup(ctx, groupID)
	if err != nil {
		return nil, &activityContract.ActivityError{Op: opFindGroup, Err: err}
	}

	// Set group ID
	schedule.ActivityGroupID = groupID
	schedule.SetTenantID(group.GetTenantID())

	// Validate the schedule
	if err := schedule.Validate(); err != nil {
		return nil, &activityContract.ActivityError{Op: opValidateSchedule, Err: err}
	}

	// Create the schedule
	if err := createLegacySchedule(ctx, s.scheduleRepo, schedule); err != nil {
		return nil, &activityContract.ActivityError{Op: "create schedule", Err: err}
	}

	return schedule, nil
}

// GetSchedule retrieves a schedule by ID
func (s *ActivityService) GetSchedule(ctx context.Context, id int64) (*activityContract.Schedule, error) {
	value, err := s.scheduleRepo.FindSchedule(ctx, id)
	if err != nil {
		err = legacyActivityScheduleError("find by id", err)
		// Convert "no rows" (bare or DatabaseError-wrapped) to our own error
		if isRepositoryNotFound(err) {
			return nil, &activityContract.ActivityError{Op: opGetSchedule, Err: activityContract.ErrScheduleNotFound}
		}
		return nil, &activityContract.ActivityError{Op: opGetSchedule, Err: err}
	}

	return legacySchedule(value), nil
}

// GetGroupSchedules retrieves all schedules for a group
func (s *ActivityService) GetGroupSchedules(ctx context.Context, groupID int64) ([]*activityContract.Schedule, error) {
	values, err := s.scheduleRepo.ListSchedules(ctx, timetable.ScheduleFilter{GroupIDs: []int64{groupID}})
	if err != nil {
		return nil, &activityContract.ActivityError{Op: "get group schedules", Err: legacyActivityScheduleError("find by group ID", err)}
	}

	return legacySchedules(values), nil
}

// DeleteSchedule deletes a schedule
func (s *ActivityService) DeleteSchedule(ctx context.Context, id int64) error {
	// Check if schedule exists
	schedule, err := findLegacySchedule(ctx, s.scheduleRepo, id)
	if err != nil {
		if isRepositoryNotFound(err) {
			return &activityContract.ActivityError{Op: "delete schedule", Err: activityContract.ErrScheduleNotFound}
		}
		return &activityContract.ActivityError{Op: "delete schedule", Err: err}
	}
	if _, err := s.findMutableActivityGroup(ctx, schedule.ActivityGroupID); err != nil {
		return &activityContract.ActivityError{Op: "delete schedule", Err: err}
	}

	// Delete the schedule
	if err := deleteLegacySchedule(ctx, s.scheduleRepo, id); err != nil {
		return &activityContract.ActivityError{Op: "delete schedule", Err: err}
	}

	return nil
}

// UpdateSchedule updates an existing schedule
func (s *ActivityService) UpdateSchedule(ctx context.Context, schedule *activityContract.Schedule) (*activityContract.Schedule, error) {
	// Check if schedule exists
	existingSchedule, err := findLegacySchedule(ctx, s.scheduleRepo, schedule.ID)
	if err != nil {
		if isRepositoryNotFound(err) {
			return nil, &activityContract.ActivityError{Op: opUpdateSchedule, Err: activityContract.ErrScheduleNotFound}
		}
		return nil, &activityContract.ActivityError{Op: opUpdateSchedule, Err: err}
	}
	if _, err := s.findMutableActivityGroup(ctx, existingSchedule.ActivityGroupID); err != nil {
		return nil, &activityContract.ActivityError{Op: opUpdateSchedule, Err: err}
	}

	// Validate the schedule
	if err := schedule.Validate(); err != nil {
		return nil, &activityContract.ActivityError{Op: opUpdateSchedule, Err: err}
	}

	// Make sure the relationship to group is preserved
	if schedule.ActivityGroupID != existingSchedule.ActivityGroupID {
		return nil, &activityContract.ActivityError{Op: opUpdateSchedule, Err: errors.New("cannot change activity group for a schedule")}
	}

	// Update the schedule
	if err := updateLegacySchedule(ctx, s.scheduleRepo, schedule); err != nil {
		return nil, &activityContract.ActivityError{Op: opUpdateSchedule, Err: err}
	}

	// Get the updated schedule
	updatedSchedule, err := findLegacySchedule(ctx, s.scheduleRepo, schedule.ID)
	if err != nil {
		return nil, &activityContract.ActivityError{Op: opUpdateSchedule, Err: err}
	}

	return updatedSchedule, nil
}
