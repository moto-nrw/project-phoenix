package activities

import (
	"context"
	"database/sql"
	"errors"

	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// ======== Schedule Methods ========

// AddSchedule adds a schedule to an activity group
func (s *Service) AddSchedule(ctx context.Context, groupID int64, schedule *activities.Schedule) (*activities.Schedule, error) {
	_, err := s.findMutableActivityGroup(ctx, groupID)
	if err != nil {
		return nil, &ActivityError{Op: opFindGroup, Err: err}
	}

	// Set group ID
	schedule.ActivityGroupID = groupID

	// Validate the schedule
	if err := schedule.Validate(); err != nil {
		return nil, &ActivityError{Op: opValidateSchedule, Err: err}
	}

	// Create the schedule
	schedule.SetTenantID(tenant.FromContext(ctx))
	if err := s.scheduleRepo.Create(ctx, schedule); err != nil {
		return nil, &ActivityError{Op: "create schedule", Err: err}
	}

	return schedule, nil
}

// GetSchedule retrieves a schedule by ID
func (s *Service) GetSchedule(ctx context.Context, id int64) (*activities.Schedule, error) {
	schedule, err := s.scheduleRepo.FindByID(ctx, id)
	if err != nil {
		// Convert "no rows" (bare or DatabaseError-wrapped) to our own error
		if base.IsNoRows(err) {
			return nil, &ActivityError{Op: opGetSchedule, Err: ErrScheduleNotFound}
		}
		return nil, &ActivityError{Op: opGetSchedule, Err: err}
	}

	return schedule, nil
}

// GetGroupSchedules retrieves all schedules for a group
func (s *Service) GetGroupSchedules(ctx context.Context, groupID int64) ([]*activities.Schedule, error) {
	schedules, err := s.scheduleRepo.FindByGroupID(ctx, groupID)
	if err != nil {
		return nil, &ActivityError{Op: "get group schedules", Err: err}
	}

	return schedules, nil
}

// DeleteSchedule deletes a schedule
func (s *Service) DeleteSchedule(ctx context.Context, id int64) error {
	// Check if schedule exists
	schedule, err := s.scheduleRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &ActivityError{Op: "delete schedule", Err: ErrScheduleNotFound}
		}
		return &ActivityError{Op: "delete schedule", Err: err}
	}
	if _, err := s.findMutableActivityGroup(ctx, schedule.ActivityGroupID); err != nil {
		return &ActivityError{Op: "delete schedule", Err: err}
	}

	// Delete the schedule
	if err := s.scheduleRepo.Delete(ctx, id); err != nil {
		return &ActivityError{Op: "delete schedule", Err: err}
	}

	return nil
}

// UpdateSchedule updates an existing schedule
func (s *Service) UpdateSchedule(ctx context.Context, schedule *activities.Schedule) (*activities.Schedule, error) {
	// Check if schedule exists
	existingSchedule, err := s.scheduleRepo.FindByID(ctx, schedule.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &ActivityError{Op: opUpdateSchedule, Err: ErrScheduleNotFound}
		}
		return nil, &ActivityError{Op: opUpdateSchedule, Err: err}
	}
	if _, err := s.findMutableActivityGroup(ctx, existingSchedule.ActivityGroupID); err != nil {
		return nil, &ActivityError{Op: opUpdateSchedule, Err: err}
	}

	// Validate the schedule
	if err := schedule.Validate(); err != nil {
		return nil, &ActivityError{Op: opUpdateSchedule, Err: err}
	}

	// Make sure the relationship to group is preserved
	if schedule.ActivityGroupID != existingSchedule.ActivityGroupID {
		return nil, &ActivityError{Op: opUpdateSchedule, Err: errors.New("cannot change activity group for a schedule")}
	}

	// Update the schedule
	if err := s.scheduleRepo.Update(ctx, schedule); err != nil {
		return nil, &ActivityError{Op: opUpdateSchedule, Err: err}
	}

	// Get the updated schedule
	updatedSchedule, err := s.scheduleRepo.FindByID(ctx, schedule.ID)
	if err != nil {
		return nil, &ActivityError{Op: opUpdateSchedule, Err: err}
	}

	return updatedSchedule, nil
}
