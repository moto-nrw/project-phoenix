package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
)

// The materializer, the conflict reads and the instance lifecycle still
// compute on the legacy exception row with time.Time clocks; these convert
// the owner's activity exception.

// exceptionsInRange lists the exceptions of [from, to] in date order, narrowed
// by filter, as legacy rows. Read and conversion failures carry the
// operation name the retained readers reported.
func exceptionsInRange(ctx context.Context, exceptions timetable.ActivityExceptionQuery, filter timetable.ActivityExceptionFilter, from, to, operation string) ([]*scheduleModels.ActivityException, error) {
	filter.FromDate, filter.ToDate, filter.OrderByDate = &from, &to, true
	values, err := exceptions.ListActivityExceptions(ctx, filter)
	if err != nil {
		return nil, WrapDatabaseError(operation, err)
	}
	rows, err := legacyActivityExceptions(values)
	if err != nil {
		return nil, WrapDatabaseError(operation, err)
	}
	return rows, nil
}

// legacyActivityExceptions converts listed exceptions; a row whose date or
// clocks do not parse fails the whole list.
func legacyActivityExceptions(values []timetable.ActivityException) ([]*scheduleModels.ActivityException, error) {
	result := make([]*scheduleModels.ActivityException, 0, len(values))
	for _, value := range values {
		row, err := legacyActivityException(value)
		if err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, nil
}

func legacyActivityException(value timetable.ActivityException) (*scheduleModels.ActivityException, error) {
	result := &scheduleModels.ActivityException{}
	if err := replaceLegacyActivityException(result, value); err != nil {
		return nil, err
	}
	return result, nil
}

func replaceLegacyActivityException(result *scheduleModels.ActivityException, value timetable.ActivityException) error {
	date, err := ParseActivityExceptionDate(value.ExceptionDate)
	if err != nil {
		return fmt.Errorf("parse activity exception date: %w", err)
	}
	start, err := legacyOptionalClock(value.StartTime)
	if err != nil {
		return fmt.Errorf("parse activity exception start time: %w", err)
	}
	end, err := legacyOptionalClock(value.EndTime)
	if err != nil {
		return fmt.Errorf("parse activity exception end time: %w", err)
	}
	*result = scheduleModels.ActivityException{
		ActivityGroupID: value.ActivityGroupID, ExceptionDate: date, ExceptionType: value.ExceptionType,
		StartTime: start, EndTime: end, RoomID: value.RoomID, Reason: value.Reason, CreatedBy: value.CreatedBy,
	}
	result.ID, result.CreatedAt, result.UpdatedAt = value.ID, value.CreatedAt, value.UpdatedAt
	result.SetTenantID(value.TenantID)
	return nil
}

func legacyOptionalClock(value *string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	parsed, err := time.Parse("15:04:05", *value)
	return &parsed, err
}

func publicActivityExceptionInput(value *scheduleModels.ActivityException) timetable.ActivityExceptionInput {
	return timetable.ActivityExceptionInput{
		ActivityGroupID: value.ActivityGroupID, ExceptionDate: value.ExceptionDate.String(),
		ExceptionType: value.ExceptionType, StartTime: publicOptionalClock(value.StartTime),
		EndTime: publicOptionalClock(value.EndTime), RoomID: value.RoomID, Reason: value.Reason, CreatedBy: value.CreatedBy,
	}
}

func publicOptionalClock(value *time.Time) *string {
	if value == nil {
		return nil
	}
	text := PublicClock(*value)
	return &text
}

func legacyActivityExceptionError(operation string, err error) error {
	if errors.Is(err, timetable.ErrActivityExceptionNotFound) {
		return WrapNotFoundDatabaseError(operation)
	}
	return WrapDatabaseError(operation, err)
}
