package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	activitiesModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/activityrecords"
)

// The activity service, the template writes, the materializer and the
// conflict reads still compute on the legacy schedule row. They call the
// owner's schedule capability through these reads and writes, which convert
// the rows and report failures under the operation names the retained
// repository used.

func groupSchedules(ctx context.Context, schedules timetable.ScheduleQuery, groupID int64) ([]*activityContract.Schedule, error) {
	values, err := schedules.ListSchedules(ctx, timetable.ScheduleFilter{GroupIDs: []int64{groupID}})
	if err != nil {
		return nil, legacyActivityScheduleError("find by group ID", err)
	}
	return legacySchedules(values), nil
}

func groupsSchedules(ctx context.Context, schedules timetable.ScheduleQuery, groupIDs []int64) ([]*activityContract.Schedule, error) {
	if len(groupIDs) == 0 {
		return []*activityContract.Schedule{}, nil
	}
	values, err := schedules.ListSchedules(ctx, timetable.ScheduleFilter{GroupIDs: groupIDs})
	if err != nil {
		return nil, legacyActivityScheduleError("find by group IDs", err)
	}
	return legacySchedules(values), nil
}

func findLegacySchedule(ctx context.Context, schedules timetable.ScheduleQuery, id int64) (*activityContract.Schedule, error) {
	value, err := schedules.FindSchedule(ctx, id)
	if err != nil {
		return nil, legacyActivityScheduleError("find by id", err)
	}
	return legacySchedule(value), nil
}

func createLegacySchedule(ctx context.Context, schedules timetable.ScheduleCommand, schedule *activityContract.Schedule) error {
	if schedule == nil {
		return errors.New("schedule cannot be nil or zero value")
	}
	if err := schedule.Validate(); err != nil {
		return err
	}
	created, err := schedules.CreateSchedule(ctx, publicScheduleInput(schedule))
	if err != nil {
		return legacyDatabaseError("create", err)
	}
	*schedule = *legacySchedule(created)
	return nil
}

func updateLegacySchedule(ctx context.Context, schedules timetable.ScheduleCommand, schedule *activityContract.Schedule) error {
	if schedule == nil {
		return errors.New("schedule cannot be nil")
	}
	if err := schedule.Validate(); err != nil {
		return err
	}
	updated, err := schedules.UpdateSchedule(ctx, schedule.ID, publicScheduleInput(schedule))
	if err != nil {
		return legacyActivityScheduleError("update", err)
	}
	*schedule = *legacySchedule(updated)
	return nil
}

func deleteLegacySchedule(ctx context.Context, schedules timetable.ScheduleCommand, id int64) error {
	if err := schedules.DeleteSchedule(ctx, id); err != nil {
		return legacyDatabaseError("delete", err)
	}
	return nil
}

func deleteLegacyGroupSchedules(ctx context.Context, schedules timetable.ScheduleCommand, groupID int64) error {
	if err := schedules.DeleteSchedulesByGroup(ctx, groupID); err != nil {
		return legacyDatabaseError("delete by group ID", err)
	}
	return nil
}

func capLegacySchedules(ctx context.Context, schedules timetable.ScheduleCommand, groupID int64, validUntil string) (int64, error) {
	rows, err := schedules.CapScheduleValidUntil(ctx, groupID, validUntil)
	if err != nil {
		return 0, legacyActivityScheduleError("cap schedule valid_until", err)
	}
	return rows, nil
}

func templateStartTimes(ctx context.Context, schedules timetable.ScheduleQuery, groupIDs []int64) ([]*activitiesModels.TemplateStartTime, error) {
	values, err := schedules.FindTemplateStartTimes(ctx, groupIDs)
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

func legacySchedules(values []timetable.Schedule) []*activityContract.Schedule {
	result := make([]*activityContract.Schedule, 0, len(values))
	for _, value := range values {
		result = append(result, legacySchedule(value))
	}
	return result
}

func legacySchedule(value timetable.Schedule) *activityContract.Schedule {
	result := &activityContract.Schedule{
		Weekday: value.Weekday, TimeframeID: value.TimeframeID, ActivityGroupID: value.ActivityGroupID,
		WeekPattern: value.WeekPattern, CalendarPeriodID: value.CalendarPeriodID,
	}
	result.SetValidityDateStrings(value.ValidUntil, value.ValidFrom)
	result.ID, result.CreatedAt, result.UpdatedAt = value.ID, value.CreatedAt, value.UpdatedAt
	result.SetTenantID(value.TenantID)
	return result
}

func publicScheduleInput(value *activityContract.Schedule) timetable.ScheduleInput {
	validUntil, validFrom := value.ValidityDateStrings()
	return timetable.ScheduleInput{
		Weekday: value.Weekday, TimeframeID: value.TimeframeID, ActivityGroupID: value.ActivityGroupID,
		WeekPattern: value.WeekPattern, CalendarPeriodID: value.CalendarPeriodID,
		ValidUntil: validUntil, ValidFrom: validFrom,
	}
}

func legacyActivityScheduleError(operation string, err error) error {
	if errors.Is(err, timetable.ErrScheduleNotFound) || errors.Is(err, timetable.ErrInvalidScheduleQuery) {
		return legacyNotFoundError(operation)
	}
	return legacyDatabaseError(operation, err)
}
