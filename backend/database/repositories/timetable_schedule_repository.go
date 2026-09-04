package repositories

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	activitiesRepo "github.com/moto-nrw/project-phoenix/database/repositories/activities"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// timetableActivityScheduleRepository preserves the legacy service contract
// while making the Timetable module the only activities.schedules provider.
type timetableActivityScheduleRepository struct{ timetable timetable.ScheduleCapability }

func (r timetableActivityScheduleRepository) Create(ctx context.Context, schedule *activitiesModels.Schedule) error {
	if schedule == nil {
		return errors.New("schedule cannot be nil or zero value")
	}
	if err := schedule.Validate(); err != nil {
		return err
	}
	created, err := r.timetable.CreateSchedule(ctx, publicScheduleInput(schedule))
	if err != nil {
		return activitiesRepo.WrapDatabaseError("create", err)
	}
	*schedule = *legacySchedule(created)
	return nil
}

func (r timetableActivityScheduleRepository) FindByID(ctx context.Context, id any) (*activitiesModels.Schedule, error) {
	scheduleID, ok := legacyGroupID(id)
	if !ok {
		return nil, activitiesRepo.WrapDatabaseError("find by id", fmt.Errorf("invalid activity schedule id %T", id))
	}
	value, err := r.timetable.FindSchedule(ctx, scheduleID)
	if err != nil {
		return nil, legacyActivityScheduleError("find by id", err)
	}
	return legacySchedule(value), nil
}

func (r timetableActivityScheduleRepository) Update(ctx context.Context, schedule *activitiesModels.Schedule) error {
	if schedule == nil {
		return errors.New("schedule cannot be nil")
	}
	if err := schedule.Validate(); err != nil {
		return err
	}
	updated, err := r.timetable.UpdateSchedule(ctx, schedule.ID, publicScheduleInput(schedule))
	if err != nil {
		return legacyActivityScheduleError("update", err)
	}
	*schedule = *legacySchedule(updated)
	return nil
}

func (r timetableActivityScheduleRepository) Delete(ctx context.Context, id any) error {
	scheduleID, ok := legacyGroupID(id)
	if !ok {
		return activitiesRepo.WrapDatabaseError("delete", fmt.Errorf("invalid activity schedule id %T", id))
	}
	if err := r.timetable.DeleteSchedule(ctx, scheduleID); err != nil {
		return activitiesRepo.WrapDatabaseError("delete", err)
	}
	return nil
}

func (r timetableActivityScheduleRepository) FindByGroupID(ctx context.Context, groupID int64) ([]*activitiesModels.Schedule, error) {
	values, err := r.timetable.ListSchedules(ctx, timetable.ScheduleFilter{GroupIDs: []int64{groupID}})
	if err != nil {
		return nil, legacyActivityScheduleError("find by group ID", err)
	}
	return legacySchedules(values), nil
}

func (r timetableActivityScheduleRepository) FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*activitiesModels.Schedule, error) {
	if len(groupIDs) == 0 {
		return []*activitiesModels.Schedule{}, nil
	}
	values, err := r.timetable.ListSchedules(ctx, timetable.ScheduleFilter{GroupIDs: groupIDs})
	if err != nil {
		return nil, legacyActivityScheduleError("find by group IDs", err)
	}
	return legacySchedules(values), nil
}

func (r timetableActivityScheduleRepository) FindByWeekday(ctx context.Context, weekday string) ([]*activitiesModels.Schedule, error) {
	value, err := strconv.Atoi(weekday)
	if err != nil {
		return nil, activitiesRepo.WrapDatabaseError("find by weekday", err)
	}
	values, err := r.timetable.ListSchedules(ctx, timetable.ScheduleFilter{Weekday: &value})
	if err != nil {
		return nil, legacyActivityScheduleError("find by weekday", err)
	}
	return legacySchedules(values), nil
}

func (r timetableActivityScheduleRepository) DeleteByGroupID(ctx context.Context, groupID int64) error {
	if err := r.timetable.DeleteSchedulesByGroup(ctx, groupID); err != nil {
		return activitiesRepo.WrapDatabaseError("delete by group ID", err)
	}
	return nil
}

func (r timetableActivityScheduleRepository) FindTemplateStartTimesByGroupIDs(ctx context.Context, groupIDs []int64) ([]*activitiesModels.TemplateStartTime, error) {
	values, err := r.timetable.FindTemplateStartTimes(ctx, groupIDs)
	if err != nil {
		return nil, legacyActivityScheduleError("find template start times by group ids", err)
	}
	result := make([]*activitiesModels.TemplateStartTime, 0, len(values))
	for _, value := range values {
		result = append(result, &activitiesModels.TemplateStartTime{
			ActivityGroupID: value.ActivityGroupID, Weekday: value.Weekday, StartTime: value.StartTime,
		})
	}
	return result, nil
}

func (r timetableActivityScheduleRepository) CapValidUntil(ctx context.Context, groupID int64, validUntil string) (int64, error) {
	rows, err := r.timetable.CapScheduleValidUntil(ctx, groupID, validUntil)
	if err != nil {
		return 0, legacyActivityScheduleError("cap schedule valid_until", err)
	}
	return rows, nil
}

func legacySchedules(values []timetable.Schedule) []*activitiesModels.Schedule {
	result := make([]*activitiesModels.Schedule, 0, len(values))
	for _, value := range values {
		result = append(result, legacySchedule(value))
	}
	return result
}

func legacySchedule(value timetable.Schedule) *activitiesModels.Schedule {
	result := &activitiesModels.Schedule{
		Weekday: value.Weekday, TimeframeID: value.TimeframeID, ActivityGroupID: value.ActivityGroupID,
		WeekPattern: value.WeekPattern, CalendarPeriodID: value.CalendarPeriodID,
	}
	result.SetValidityDateStrings(value.ValidUntil, value.ValidFrom)
	result.ID, result.CreatedAt, result.UpdatedAt = value.ID, value.CreatedAt, value.UpdatedAt
	result.SetTenantID(value.TenantID)
	return result
}

func publicScheduleInput(value *activitiesModels.Schedule) timetable.ScheduleInput {
	validUntil, validFrom := value.ValidityDateStrings()
	return timetable.ScheduleInput{
		Weekday: value.Weekday, TimeframeID: value.TimeframeID, ActivityGroupID: value.ActivityGroupID,
		WeekPattern: value.WeekPattern, CalendarPeriodID: value.CalendarPeriodID,
		ValidUntil: validUntil, ValidFrom: validFrom,
	}
}

func legacyActivityScheduleError(operation string, err error) error {
	if errors.Is(err, timetable.ErrScheduleNotFound) || errors.Is(err, timetable.ErrInvalidScheduleQuery) {
		return activitiesRepo.WrapNotFoundDatabaseError(operation)
	}
	return activitiesRepo.WrapDatabaseError(operation, err)
}
