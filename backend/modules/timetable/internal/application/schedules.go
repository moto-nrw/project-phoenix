package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (s *Service) FindSchedule(ctx context.Context, id int64) (result timetable.Schedule, err error) {
	if id <= 0 {
		return timetable.Schedule{}, s.reject("find_schedule", timetable.ErrInvalidScheduleQuery)
	}

	err = s.run(ctx, "find_schedule", func(ctx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindSchedule(ctx, id)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return timetable.ErrScheduleNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) ListSchedules(ctx context.Context, filter timetable.ScheduleFilter) (result []timetable.Schedule, err error) {
	err = s.run(ctx, "list_schedules", func(ctx context.Context, stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListSchedules(ctx, filter)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) FindTemplateStartTimes(ctx context.Context, groupIDs []int64) (result []timetable.TemplateStartTime, err error) {
	err = s.run(ctx, "find_template_start_times", func(ctx context.Context, stats *domain.OperationStats) error {
		values, queryStats, findErr := s.store.FindTemplateStartTimes(ctx, groupIDs)
		stats.Add(queryStats)
		result = values
		return findErr
	})
	return result, err
}

func (s *Service) CreateSchedule(ctx context.Context, fields timetable.ScheduleInput) (result timetable.Schedule, err error) {
	if !normalizeSchedule(&fields) {
		return timetable.Schedule{}, s.reject("create_schedule", timetable.ErrInvalidSchedule)
	}

	err = s.runWrite(ctx, "create_schedule", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, queryStats, createErr := s.store.CreateSchedule(txCtx, fields)
		stats.Add(queryStats)
		result = value
		return createErr
	})
	return result, err
}

func (s *Service) UpdateSchedule(ctx context.Context, id int64, fields timetable.ScheduleInput) (result timetable.Schedule, err error) {
	if id <= 0 || !normalizeSchedule(&fields) {
		return timetable.Schedule{}, s.reject("update_schedule", timetable.ErrInvalidSchedule)
	}

	err = s.runWrite(ctx, "update_schedule", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, updateErr := s.store.UpdateSchedule(txCtx, id, fields)
		stats.Add(queryStats)
		if updateErr != nil {
			return updateErr
		}
		if !found {
			return timetable.ErrScheduleNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) DeleteSchedule(ctx context.Context, id int64) error {
	if id <= 0 {
		return s.reject("delete_schedule", timetable.ErrInvalidSchedule)
	}

	return s.runWrite(ctx, "delete_schedule", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeleteSchedule(txCtx, id)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) DeleteSchedulesByGroup(ctx context.Context, groupID int64) error {
	if groupID <= 0 {
		return s.reject("delete_schedules_by_group", timetable.ErrInvalidSchedule)
	}

	return s.runWrite(ctx, "delete_schedules_by_group", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeleteSchedulesByGroup(txCtx, groupID)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) CapScheduleValidUntil(ctx context.Context, groupID int64, validUntil string) (result int64, err error) {
	if groupID <= 0 || !timetable.ValidDate(validUntil) {
		return 0, s.reject("cap_schedule_valid_until", timetable.ErrInvalidSchedule)
	}

	err = s.runWrite(ctx, "cap_schedule_valid_until", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		rows, queryStats, capErr := s.store.CapScheduleValidUntil(txCtx, groupID, validUntil)
		stats.Add(queryStats)
		result = rows
		return capErr
	})
	return result, err
}
