package application

import (
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
)

// The template writes and the materializer still compute on the legacy
// timeframe row with time.Time clocks; these convert the owner's timeframe.

// legacyTimeframes converts listed timeframes; a row whose clocks do not
// parse fails the whole list.
func legacyTimeframes(values []timetable.Timeframe) ([]*scheduleModels.Timeframe, error) {
	result := make([]*scheduleModels.Timeframe, 0, len(values))
	for _, value := range values {
		row, err := legacyTimeframe(value)
		if err != nil {
			return nil, err
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
		clock := PublicClock(*value.EndTime)
		end = &clock
	}
	return timetable.TimeframeInput{StartTime: PublicClock(value.StartTime), EndTime: end,
		IsActive: value.IsActive, Description: value.Description}
}

func PublicClock(value time.Time) string { return value.Format("15:04:05.999999999") }

func legacyTimeframeError(operation string, err error) error {
	if errors.Is(err, timetable.ErrTimeframeNotFound) {
		return WrapNotFoundDatabaseError(operation)
	}
	return WrapDatabaseError(operation, err)
}
