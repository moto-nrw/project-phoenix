package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

type timetableTimeframeRepository struct{ timetable timetable.TimeframeCapability }

func (r timetableTimeframeRepository) Create(ctx context.Context, value *scheduleModels.Timeframe) error {
	if value == nil {
		return errors.New("Timeframe cannot be nil or zero value")
	}
	if err := value.Validate(); err != nil {
		return err
	}
	created, err := r.timetable.CreateTimeframe(ctx, publicTimeframeInput(value))
	if err != nil {
		return scheduleRepo.WrapDatabaseError("create", err)
	}
	return replaceLegacyTimeframe(value, created)
}

func (r timetableTimeframeRepository) FindByID(ctx context.Context, id any) (*scheduleModels.Timeframe, error) {
	timeframeID, ok := legacyGroupID(id)
	if !ok {
		return nil, scheduleRepo.WrapDatabaseError("find by id", fmt.Errorf("invalid timeframe id %T", id))
	}
	value, err := r.timetable.FindTimeframe(ctx, timeframeID)
	if err != nil {
		return nil, legacyTimeframeError("find by id", err)
	}
	return legacyTimeframe(value)
}

func (r timetableTimeframeRepository) Update(ctx context.Context, value *scheduleModels.Timeframe) error {
	if value == nil {
		return errors.New("Timeframe cannot be nil or zero value")
	}
	if err := value.Validate(); err != nil {
		return err
	}
	updated, err := r.timetable.UpdateTimeframe(ctx, value.ID, publicTimeframeInput(value))
	if err != nil {
		return legacyTimeframeError("update", err)
	}
	return replaceLegacyTimeframe(value, updated)
}

func (r timetableTimeframeRepository) Delete(ctx context.Context, id any) error {
	timeframeID, ok := legacyGroupID(id)
	if !ok {
		return scheduleRepo.WrapDatabaseError("delete", fmt.Errorf("invalid timeframe id %T", id))
	}
	if err := r.timetable.DeleteTimeframe(ctx, timeframeID); err != nil {
		return scheduleRepo.WrapDatabaseError("delete", err)
	}
	return nil
}

func (r timetableTimeframeRepository) List(ctx context.Context, options *scheduleModels.TimeframeQueryOptions) ([]*scheduleModels.Timeframe, error) {
	description, limit, offset, err := scheduleRepo.TimeframeListOptions(options)
	if err != nil {
		return nil, scheduleRepo.WrapDatabaseError("list with options", err)
	}
	description = strings.TrimSuffix(strings.TrimPrefix(description, "%"), "%")
	return r.list(ctx, timetable.TimeframeFilter{DescriptionContains: description, Limit: limit, Offset: offset}, "list with options")
}

func (r timetableTimeframeRepository) FindActive(ctx context.Context) ([]*scheduleModels.Timeframe, error) {
	return r.list(ctx, timetable.TimeframeFilter{ActiveOnly: true}, "find active")
}

func (r timetableTimeframeRepository) FindByTimeRange(ctx context.Context, startTime, endTime time.Time) ([]*scheduleModels.Timeframe, error) {
	start, end := publicClock(startTime), publicClock(endTime)
	return r.list(ctx, timetable.TimeframeFilter{OverlapsStart: &start, OverlapsEnd: &end}, "find by time range")
}

func (r timetableTimeframeRepository) list(ctx context.Context, filter timetable.TimeframeFilter, operation string) ([]*scheduleModels.Timeframe, error) {
	values, err := r.timetable.ListTimeframes(ctx, filter)
	if err != nil {
		return nil, legacyTimeframeError(operation, err)
	}
	result := make([]*scheduleModels.Timeframe, 0, len(values))
	for _, value := range values {
		row, convertErr := legacyTimeframe(value)
		if convertErr != nil {
			return nil, scheduleRepo.WrapDatabaseError(operation, convertErr)
		}
		result = append(result, row)
	}
	return result, nil
}

func legacyTimeframe(value timetable.Timeframe) (*scheduleModels.Timeframe, error) {
	result := &scheduleModels.Timeframe{}
	if err := replaceLegacyTimeframe(result, value); err != nil {
		return nil, err
	}
	return result, nil
}

func replaceLegacyTimeframe(result *scheduleModels.Timeframe, value timetable.Timeframe) error {
	start, err := time.Parse("15:04:05", value.StartTime)
	if err != nil {
		return fmt.Errorf("parse timeframe start time: %w", err)
	}
	var end *time.Time
	if value.EndTime != nil {
		parsed, parseErr := time.Parse("15:04:05", *value.EndTime)
		if parseErr != nil {
			return fmt.Errorf("parse timeframe end time: %w", parseErr)
		}
		end = &parsed
	}
	*result = scheduleModels.Timeframe{StartTime: start, EndTime: end, IsActive: value.IsActive, Description: value.Description}
	result.ID, result.CreatedAt, result.UpdatedAt = value.ID, value.CreatedAt, value.UpdatedAt
	result.SetTenantID(value.TenantID)
	return nil
}

func publicTimeframeInput(value *scheduleModels.Timeframe) timetable.TimeframeInput {
	var end *string
	if value.EndTime != nil {
		clock := publicClock(*value.EndTime)
		end = &clock
	}
	return timetable.TimeframeInput{StartTime: publicClock(value.StartTime), EndTime: end,
		IsActive: value.IsActive, Description: value.Description}
}

func publicClock(value time.Time) string { return value.Format("15:04:05.999999999") }

func legacyTimeframeError(operation string, err error) error {
	if errors.Is(err, timetable.ErrTimeframeNotFound) {
		return scheduleRepo.WrapNotFoundDatabaseError(operation)
	}
	return scheduleRepo.WrapDatabaseError(operation, err)
}
