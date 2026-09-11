package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

const legacyActivityExceptionDateColumn = "exception_date"

type timetableActivityExceptionRepository struct {
	timetable timetable.ActivityExceptionCapability
}

func (r timetableActivityExceptionRepository) Create(ctx context.Context, value *scheduleModels.ActivityException) error {
	if value == nil {
		return errors.New("activity exception cannot be nil or zero value")
	}
	if err := value.Validate(); err != nil {
		return err
	}
	created, err := r.timetable.CreateActivityException(ctx, publicActivityExceptionInput(value))
	if err != nil {
		return scheduleRepo.WrapDatabaseError("create", err)
	}
	return replaceLegacyActivityException(value, created)
}

func (r timetableActivityExceptionRepository) FindByID(ctx context.Context, id any) (*scheduleModels.ActivityException, error) {
	exceptionID, ok := legacyGroupID(id)
	if !ok {
		return nil, scheduleRepo.WrapDatabaseError("find by id", fmt.Errorf("invalid activity exception id %T", id))
	}
	value, err := r.timetable.FindActivityException(ctx, exceptionID)
	if err != nil {
		return nil, legacyActivityExceptionError("find by id", err)
	}
	return legacyActivityException(value)
}

func (r timetableActivityExceptionRepository) Update(ctx context.Context, value *scheduleModels.ActivityException) error {
	if value == nil {
		return errors.New("activity exception cannot be nil or zero value")
	}
	if err := value.Validate(); err != nil {
		return err
	}
	updated, err := r.timetable.UpdateActivityException(ctx, value.ID, publicActivityExceptionInput(value))
	if err != nil {
		return legacyActivityExceptionError("update", err)
	}
	return replaceLegacyActivityException(value, updated)
}

func (r timetableActivityExceptionRepository) Delete(ctx context.Context, id any) error {
	exceptionID, ok := legacyGroupID(id)
	if !ok {
		return scheduleRepo.WrapDatabaseError("delete", fmt.Errorf("invalid activity exception id %T", id))
	}
	if err := r.timetable.DeleteActivityException(ctx, exceptionID); err != nil {
		return scheduleRepo.WrapDatabaseError("delete", err)
	}
	return nil
}

func (r timetableActivityExceptionRepository) List(ctx context.Context, options *scheduleRepo.ActivityExceptionQueryOptions) ([]*scheduleModels.ActivityException, error) {
	groupID, limit, offset, err := scheduleRepo.ActivityExceptionListOptions(options)
	if err != nil {
		return nil, scheduleRepo.WrapDatabaseError("list with options", err)
	}
	return r.list(ctx, timetable.ActivityExceptionFilter{
		ActivityGroupID: groupID, Limit: limit, Offset: offset,
	}, "list with options")
}

func (r timetableActivityExceptionRepository) FindByActivityGroupID(ctx context.Context, groupID int64) ([]*scheduleModels.ActivityException, error) {
	return r.list(ctx, timetable.ActivityExceptionFilter{ActivityGroupID: &groupID, OrderByDate: true}, "find by activity group id")
}

func (r timetableActivityExceptionRepository) FindByActivityGroupAndDate(ctx context.Context, groupID int64, date scheduleRepo.ActivityExceptionDate) (*scheduleModels.ActivityException, error) {
	text := date.String()
	values, err := r.timetable.ListActivityExceptions(ctx, timetable.ActivityExceptionFilter{
		ActivityGroupID: &groupID, ExceptionDate: &text, Limit: 1,
	})
	if err != nil {
		return nil, scheduleRepo.WrapDatabaseError("find by activity group and date", err)
	}
	if len(values) == 0 {
		return nil, nil
	}
	result, err := legacyActivityException(values[0])
	if err != nil {
		return nil, scheduleRepo.WrapDatabaseError("find by activity group and date", err)
	}
	return result, nil
}

func (r timetableActivityExceptionRepository) FindByActivityGroupAndDateRange(ctx context.Context, groupID int64, from, to scheduleRepo.ActivityExceptionDate) ([]*scheduleModels.ActivityException, error) {
	fromText, toText := from.String(), to.String()
	return r.list(ctx, timetable.ActivityExceptionFilter{
		ActivityGroupID: &groupID, FromDate: &fromText, ToDate: &toText, OrderByDate: true,
	}, "find by activity group and date range")
}

func (r timetableActivityExceptionRepository) FindByDateRange(ctx context.Context, from, to scheduleRepo.ActivityExceptionDate) ([]*scheduleModels.ActivityException, error) {
	fromText, toText := from.String(), to.String()
	return r.list(ctx, timetable.ActivityExceptionFilter{
		FromDate: &fromText, ToDate: &toText, OrderByDate: true,
	}, "find by date range")
}

func (r timetableActivityExceptionRepository) CountWithOptions(ctx context.Context, options *scheduleRepo.ActivityExceptionQueryOptions) (int, error) {
	before, err := scheduleRepo.ActivityExceptionBefore(options)
	if err != nil {
		return 0, scheduleRepo.WrapDatabaseError("count with options", err)
	}
	count, err := r.timetable.CountActivityExceptions(ctx, before)
	if err != nil {
		return 0, scheduleRepo.WrapDatabaseError("count with options", err)
	}
	return count, nil
}

func (r timetableActivityExceptionRepository) OldestBefore(ctx context.Context, column string, cutoff *scheduleRepo.ActivityExceptionDate) (*scheduleRepo.ActivityExceptionDate, error) {
	if column != legacyActivityExceptionDateColumn {
		return nil, scheduleRepo.WrapDatabaseError("oldest before", fmt.Errorf("unsupported activity exception date column %q", column))
	}
	before := publicOptionalDate(cutoff)
	value, err := r.timetable.OldestActivityExceptionBefore(ctx, before)
	if err != nil {
		return nil, scheduleRepo.WrapDatabaseError("oldest before", err)
	}
	if value == nil {
		return nil, nil
	}
	result, err := scheduleRepo.ParseActivityExceptionDate(*value)
	if err != nil {
		return nil, scheduleRepo.WrapDatabaseError("oldest before", err)
	}
	return &result, nil
}

func (r timetableActivityExceptionRepository) DeleteOlderThan(ctx context.Context, column string, cutoff scheduleRepo.ActivityExceptionDate) (int64, error) {
	if column != legacyActivityExceptionDateColumn {
		return 0, scheduleRepo.WrapDatabaseError("delete older than", fmt.Errorf("unsupported activity exception date column %q", column))
	}
	rows, err := r.timetable.DeleteActivityExceptionsBefore(ctx, cutoff.String())
	if err != nil {
		return 0, scheduleRepo.WrapDatabaseError("delete older than", err)
	}
	return rows, nil
}

func (r timetableActivityExceptionRepository) list(ctx context.Context, filter timetable.ActivityExceptionFilter, operation string) ([]*scheduleModels.ActivityException, error) {
	values, err := r.timetable.ListActivityExceptions(ctx, filter)
	if err != nil {
		return nil, scheduleRepo.WrapDatabaseError(operation, err)
	}
	result := make([]*scheduleModels.ActivityException, 0, len(values))
	for _, value := range values {
		row, convertErr := legacyActivityException(value)
		if convertErr != nil {
			return nil, scheduleRepo.WrapDatabaseError(operation, convertErr)
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
	date, err := scheduleRepo.ParseActivityExceptionDate(value.ExceptionDate)
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
	text := publicClock(*value)
	return &text
}

func publicOptionalDate(value *scheduleRepo.ActivityExceptionDate) *string {
	if value == nil {
		return nil
	}
	text := value.String()
	return &text
}

func legacyActivityExceptionError(operation string, err error) error {
	if errors.Is(err, timetable.ErrActivityExceptionNotFound) {
		return scheduleRepo.WrapNotFoundDatabaseError(operation)
	}
	return scheduleRepo.WrapDatabaseError(operation, err)
}
