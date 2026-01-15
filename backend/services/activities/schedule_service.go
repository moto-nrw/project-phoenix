package activities

import (
	"context"
	"database/sql"
	"errors"

	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// ======== Schedule Methods ========

// AddSchedule adds a schedule to an activity group
func (s *Service) AddSchedule(ctx context.Context, groupID int64, schedule *activities.Schedule) (*activities.Schedule, error) {
	// Check if group exists
	_, err := s.groupRepo.FindByID(ctx, groupID)
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
	if err := s.scheduleRepo.Create(ctx, schedule); err != nil {
		return nil, &ActivityError{Op: "create schedule", Err: err}
	}

	return schedule, nil
}

// GetSchedule retrieves a schedule by ID
func (s *Service) GetSchedule(ctx context.Context, id int64) (*activities.Schedule, error) {
	schedule, err := s.scheduleRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &ActivityError{Op: opGetSchedule, Err: ErrScheduleNotFound}
		}
		if dbErr, ok := err.(*base.DatabaseError); ok && errors.Is(dbErr.Err, sql.ErrNoRows) {
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
	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		txService := s.WithTx(tx).(ActivityService)

		// Check if schedule exists
		_, err := txService.(*Service).scheduleRepo.FindByID(ctx, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrScheduleNotFound
			}
			return &ActivityError{Op: "find schedule", Err: err}
		}

		// Delete the schedule
		if err := txService.(*Service).scheduleRepo.Delete(ctx, id); err != nil {
			return &ActivityError{Op: "delete schedule", Err: err}
		}

		return nil
	})

	if err != nil {
		return &ActivityError{Op: "delete schedule", Err: err}
	}

	return nil
}

// UpdateSchedule updates an existing schedule
func (s *Service) UpdateSchedule(ctx context.Context, schedule *activities.Schedule) (*activities.Schedule, error) {
	var result *activities.Schedule

	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		txService := s.WithTx(tx).(ActivityService)

		// Check if schedule exists
		existingSchedule, err := txService.(*Service).scheduleRepo.FindByID(ctx, schedule.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrScheduleNotFound
			}
			return &ActivityError{Op: "find schedule", Err: err}
		}

		// Validate the schedule
		if err := schedule.Validate(); err != nil {
			return &ActivityError{Op: opValidateSchedule, Err: err}
		}

		// Make sure the relationship to group is preserved
		if schedule.ActivityGroupID != existingSchedule.ActivityGroupID {
			return &ActivityError{Op: opUpdateSchedule, Err: errors.New("cannot change activity group for a schedule")}
		}

		// Update the schedule
		if err := txService.(*Service).scheduleRepo.Update(ctx, schedule); err != nil {
			return &ActivityError{Op: opUpdateSchedule, Err: err}
		}

		// Get the updated schedule
		updatedSchedule, err := txService.(*Service).scheduleRepo.FindByID(ctx, schedule.ID)
		if err != nil {
			return &ActivityError{Op: "retrieve updated schedule", Err: err}
		}

		result = updatedSchedule
		return nil
	})

	if err != nil {
		return nil, &ActivityError{Op: opUpdateSchedule, Err: err}
	}

	return result, nil
}
