package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

const legacyActivityInstanceDateColumn = "date"

type timetableActivityInstanceRepository struct {
	timetable timetable.ActivityInstanceCapability
}

func (r timetableActivityInstanceRepository) Create(ctx context.Context, value *scheduleModels.ActivityInstance) error {
	if value == nil {
		return errors.New("activity instance cannot be nil or zero value")
	}
	if err := value.Validate(); err != nil {
		return err
	}
	created, err := r.timetable.CreateActivityInstance(ctx, publicActivityInstanceInput(value))
	if err != nil {
		return scheduleRepo.WrapDatabaseError("create", err)
	}
	return replaceLegacyActivityInstance(value, created)
}

func (r timetableActivityInstanceRepository) CreateTemplateBackedIfAbsent(ctx context.Context, value *scheduleModels.ActivityInstance) (bool, error) {
	if value == nil {
		return false, errors.New("activity instance cannot be nil")
	}
	if value.ActivityGroupID == nil {
		return false, errors.New("activity_group_id is required for template-backed insert")
	}
	if err := value.Validate(); err != nil {
		return false, err
	}
	created, inserted, err := r.timetable.CreateTemplateBackedActivityInstanceIfAbsent(ctx, publicActivityInstanceInput(value))
	if err != nil {
		return false, scheduleRepo.WrapDatabaseError("create template-backed if absent", err)
	}
	if inserted {
		return true, replaceLegacyActivityInstance(value, created)
	}
	return false, nil
}

func (r timetableActivityInstanceRepository) CreateIdempotent(ctx context.Context, value *scheduleModels.ActivityInstance) (bool, error) {
	if value == nil {
		return false, errors.New("activity instance cannot be nil")
	}
	if value.IdempotencyKey == nil {
		return false, errors.New("idempotency_key is required for idempotent insert")
	}
	if err := value.Validate(); err != nil {
		return false, err
	}
	created, inserted, err := r.timetable.CreateIdempotentActivityInstance(ctx, publicActivityInstanceInput(value))
	if err != nil {
		return false, scheduleRepo.WrapDatabaseError("create idempotent", err)
	}
	if inserted {
		return true, replaceLegacyActivityInstance(value, created)
	}
	return false, nil
}

func (r timetableActivityInstanceRepository) FindByID(ctx context.Context, id any) (*scheduleModels.ActivityInstance, error) {
	instanceID, ok := legacyGroupID(id)
	if !ok {
		return nil, scheduleRepo.WrapDatabaseError("find by id", fmt.Errorf("invalid activity instance id %T", id))
	}
	value, err := r.timetable.FindActivityInstance(ctx, instanceID)
	if err != nil {
		return nil, legacyActivityInstanceError("find by id", err)
	}
	return legacyActivityInstance(value)
}

func (r timetableActivityInstanceRepository) Update(ctx context.Context, value *scheduleModels.ActivityInstance) error {
	if value == nil {
		return errors.New("activity instance cannot be nil or zero value")
	}
	if err := value.Validate(); err != nil {
		return err
	}
	updated, err := r.timetable.UpdateActivityInstance(ctx, value.ID, publicActivityInstanceInput(value))
	if err != nil {
		return legacyActivityInstanceError("update", err)
	}
	return replaceLegacyActivityInstance(value, updated)
}

func (r timetableActivityInstanceRepository) Delete(ctx context.Context, id any) error {
	instanceID, ok := legacyGroupID(id)
	if !ok {
		return scheduleRepo.WrapDatabaseError("delete", fmt.Errorf("invalid activity instance id %T", id))
	}
	if err := r.timetable.DeleteActivityInstance(ctx, instanceID); err != nil {
		return scheduleRepo.WrapDatabaseError("delete", err)
	}
	return nil
}

func (r timetableActivityInstanceRepository) List(ctx context.Context, options *scheduleRepo.ActivityInstanceQueryOptions) ([]*scheduleModels.ActivityInstance, error) {
	filter, err := scheduleRepo.ActivityInstanceListOptions(options)
	if err != nil {
		return nil, scheduleRepo.WrapDatabaseError("list with options", err)
	}
	return r.list(ctx, publicActivityInstanceFilter(filter), "list with options")
}

func (r timetableActivityInstanceRepository) FindByTenantAndDate(ctx context.Context, date scheduleRepo.ActivityInstanceDate) ([]*scheduleModels.ActivityInstance, error) {
	text := date.String()
	return r.list(ctx, timetable.ActivityInstanceFilter{Date: &text, OrderByDateAndTime: true}, "find by tenant and date")
}

func (r timetableActivityInstanceRepository) FindPlannedTemplateBackedFrom(ctx context.Context, from scheduleRepo.ActivityInstanceDate) ([]*scheduleModels.ActivityInstance, error) {
	text := from.String()
	return r.list(ctx, timetable.ActivityInstanceFilter{
		FromDate: &text, MaterializedPlanned: true, OrderByDateAndTime: true,
	}, "find planned template-backed instances from date")
}

func (r timetableActivityInstanceRepository) MaxID(ctx context.Context) (int64, error) {
	value, err := r.timetable.MaxActivityInstanceID(ctx)
	if err != nil {
		return 0, scheduleRepo.WrapDatabaseError("get max activity instance id", err)
	}
	return value, nil
}

func (r timetableActivityInstanceRepository) FindByTenantAndDateRange(ctx context.Context, from, to scheduleRepo.ActivityInstanceDate) ([]*scheduleModels.ActivityInstance, error) {
	fromText, toText := from.String(), to.String()
	return r.list(ctx, timetable.ActivityInstanceFilter{
		FromDate: &fromText, ToDate: &toText, OrderByDateAndTime: true,
	}, "find by tenant and date range")
}

func (r timetableActivityInstanceRepository) FindByIDs(ctx context.Context, ids []int64) ([]*scheduleModels.ActivityInstance, error) {
	if len(ids) == 0 {
		return []*scheduleModels.ActivityInstance{}, nil
	}
	return r.list(ctx, timetable.ActivityInstanceFilter{IDs: ids, OrderByDateAndTime: true}, "find by ids")
}

func (r timetableActivityInstanceRepository) FindByActivityGroupAndDate(ctx context.Context, groupID int64, date scheduleRepo.ActivityInstanceDate) ([]*scheduleModels.ActivityInstance, error) {
	text := date.String()
	return r.list(ctx, timetable.ActivityInstanceFilter{
		ActivityGroupID: &groupID, Date: &text, OrderByDateAndTime: true,
	}, "find by activity group and date")
}

func (r timetableActivityInstanceRepository) FindByActivityGroupAndDateRange(ctx context.Context, groupID int64, from, to scheduleRepo.ActivityInstanceDate) ([]*scheduleModels.ActivityInstance, error) {
	fromText, toText := from.String(), to.String()
	return r.list(ctx, timetable.ActivityInstanceFilter{
		ActivityGroupID: &groupID, FromDate: &fromText, ToDate: &toText, OrderByDateAndTime: true,
	}, "find by activity group and date range")
}

func (r timetableActivityInstanceRepository) FindByActiveGroupID(ctx context.Context, groupID int64) (*scheduleModels.ActivityInstance, error) {
	values, err := r.timetable.ListActivityInstances(ctx, timetable.ActivityInstanceFilter{ActiveGroupID: &groupID, Limit: 1})
	if err != nil {
		return nil, scheduleRepo.WrapDatabaseError("find by active group id", err)
	}
	if len(values) == 0 {
		return nil, nil
	}
	result, err := legacyActivityInstance(values[0])
	if err != nil {
		return nil, scheduleRepo.WrapDatabaseError("find by active group id", err)
	}
	return result, nil
}

func (r timetableActivityInstanceRepository) MarkCompleted(ctx context.Context, instanceID int64, completedAt time.Time) error {
	if err := r.timetable.MarkActivityInstanceCompleted(ctx, instanceID, completedAt); err != nil {
		return legacyActivityInstanceError("mark completed", err)
	}
	return nil
}

func (r timetableActivityInstanceRepository) CompleteActiveByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, completedAt time.Time) (int64, error) {
	rows, err := r.timetable.CompleteActiveActivityInstances(ctx, activeGroupIDs, completedAt)
	if err != nil {
		return 0, scheduleRepo.WrapDatabaseError("complete active instances by active group ids", err)
	}
	return rows, nil
}

func (r timetableActivityInstanceRepository) DeletePlannedNonSpontaneousInWindow(ctx context.Context, from scheduleRepo.ActivityInstanceDate, to *scheduleRepo.ActivityInstanceDate, groupID *int64, preserveDeviations bool) (int64, error) {
	rows, err := r.timetable.DeletePlannedActivityInstances(ctx, from.String(), publicOptionalInstanceDate(to), groupID, preserveDeviations)
	if err != nil {
		return 0, scheduleRepo.WrapDatabaseError("delete planned non-spontaneous in window", err)
	}
	return rows, nil
}

func (r timetableActivityInstanceRepository) DeletePlannedMaterializedWeekendInstances(ctx context.Context, groupID int64, weekdays []int) (int64, error) {
	rows, err := r.timetable.DeleteRemovedWeekendActivityInstances(ctx, groupID, weekdays)
	if err != nil {
		return 0, scheduleRepo.WrapDatabaseError("delete removed legacy weekend instances", err)
	}
	return rows, nil
}

func (r timetableActivityInstanceRepository) PropagateListKindToFutureInstances(ctx context.Context, groupID int64, previousKind, newKind *string, after scheduleRepo.ActivityInstanceDate) (int64, error) {
	rows, err := r.timetable.PropagateActivityInstanceListKind(ctx, groupID, previousKind, newKind, after.String())
	if err != nil {
		return 0, scheduleRepo.WrapDatabaseError("propagate list kind to future instances", err)
	}
	return rows, nil
}

func (r timetableActivityInstanceRepository) UpdateColumns(ctx context.Context, value *scheduleModels.ActivityInstance, columns ...string) (int64, error) {
	if value == nil {
		return 0, errors.New("ActivityInstance cannot be nil or zero value")
	}
	if len(columns) == 0 {
		return 0, errors.New("update columns ActivityInstance: at least one column required")
	}
	rows, err := r.timetable.PatchActivityInstance(ctx, value.ID, publicActivityInstanceInput(value), columns)
	if err != nil {
		return 0, scheduleRepo.WrapDatabaseError("update columns", err)
	}
	return rows, nil
}

func (r timetableActivityInstanceRepository) CountWithOptions(ctx context.Context, options *scheduleRepo.ActivityInstanceQueryOptions) (int, error) {
	before, err := scheduleRepo.ActivityInstanceBefore(options)
	if err != nil {
		return 0, scheduleRepo.WrapDatabaseError("count with options", err)
	}
	count, err := r.timetable.CountActivityInstances(ctx, before)
	if err != nil {
		return 0, scheduleRepo.WrapDatabaseError("count with options", err)
	}
	return count, nil
}

func (r timetableActivityInstanceRepository) OldestBefore(ctx context.Context, column string, cutoff *scheduleRepo.ActivityInstanceDate) (*scheduleRepo.ActivityInstanceDate, error) {
	if column != legacyActivityInstanceDateColumn {
		return nil, scheduleRepo.WrapDatabaseError("oldest before", fmt.Errorf("unsupported activity instance date column %q", column))
	}
	value, err := r.timetable.OldestActivityInstanceBefore(ctx, publicOptionalInstanceDate(cutoff))
	if err != nil {
		return nil, scheduleRepo.WrapDatabaseError("oldest before", err)
	}
	if value == nil {
		return nil, nil
	}
	result, err := scheduleRepo.ParseActivityInstanceDate(*value)
	if err != nil {
		return nil, scheduleRepo.WrapDatabaseError("oldest before", err)
	}
	return &result, nil
}

func (r timetableActivityInstanceRepository) DeleteOlderThan(ctx context.Context, column string, cutoff scheduleRepo.ActivityInstanceDate) (int64, error) {
	if column != legacyActivityInstanceDateColumn {
		return 0, scheduleRepo.WrapDatabaseError("delete older than", fmt.Errorf("unsupported activity instance date column %q", column))
	}
	rows, err := r.timetable.DeleteActivityInstancesBefore(ctx, cutoff.String())
	if err != nil {
		return 0, scheduleRepo.WrapDatabaseError("delete older than", err)
	}
	return rows, nil
}

func (r timetableActivityInstanceRepository) list(ctx context.Context, filter timetable.ActivityInstanceFilter, operation string) ([]*scheduleModels.ActivityInstance, error) {
	values, err := r.timetable.ListActivityInstances(ctx, filter)
	if err != nil {
		return nil, scheduleRepo.WrapDatabaseError(operation, err)
	}
	result := make([]*scheduleModels.ActivityInstance, 0, len(values))
	for _, value := range values {
		row, convertErr := legacyActivityInstance(value)
		if convertErr != nil {
			return nil, scheduleRepo.WrapDatabaseError(operation, convertErr)
		}
		result = append(result, row)
	}
	return result, nil
}

func legacyActivityInstance(value timetable.ActivityInstance) (*scheduleModels.ActivityInstance, error) {
	result := &scheduleModels.ActivityInstance{}
	if err := replaceLegacyActivityInstance(result, value); err != nil {
		return nil, err
	}
	return result, nil
}

func replaceLegacyActivityInstance(result *scheduleModels.ActivityInstance, value timetable.ActivityInstance) error {
	date, err := scheduleRepo.ParseActivityInstanceDate(value.Date)
	if err != nil {
		return fmt.Errorf("parse activity instance date: %w", err)
	}
	start, err := time.Parse("15:04:05", value.StartTime)
	if err != nil {
		return fmt.Errorf("parse activity instance start time: %w", err)
	}
	end, err := time.Parse("15:04:05", value.EndTime)
	if err != nil {
		return fmt.Errorf("parse activity instance end time: %w", err)
	}
	*result = scheduleModels.ActivityInstance{
		Date: date, ActivityGroupID: value.ActivityGroupID, CalendarPeriodID: value.CalendarPeriodID,
		Title: value.Title, Description: value.Description, StartTime: start, EndTime: end, RoomID: value.RoomID,
		RequiredStaff: value.RequiredStaff, Status: value.Status, ActiveGroupID: value.ActiveGroupID,
		ListKind: value.ListKind, IsSpontaneous: value.IsSpontaneous, UnderstaffedAck: value.UnderstaffedAck,
		UnderstaffedNote: value.UnderstaffedNote, CancelReason: value.CancelReason, Notes: value.Notes,
		IdempotencyKey: value.IdempotencyKey, IdempotencyFingerprint: value.IdempotencyFingerprint,
		CreatedBy: value.CreatedBy, StartedBy: value.StartedBy, StartedAt: value.StartedAt,
		CompletedAt: value.CompletedAt, CompletedBy: value.CompletedBy, ReopenUntil: value.ReopenUntil,
		CompletionSnapshot: json.RawMessage(value.CompletionSnapshot),
	}
	result.ID, result.CreatedAt, result.UpdatedAt = value.ID, value.CreatedAt, value.UpdatedAt
	result.SetTenantID(value.TenantID)
	return nil
}

func publicActivityInstanceInput(value *scheduleModels.ActivityInstance) timetable.ActivityInstanceInput {
	return timetable.ActivityInstanceInput{
		Date: value.Date.String(), ActivityGroupID: value.ActivityGroupID, CalendarPeriodID: value.CalendarPeriodID,
		Title: value.Title, Description: value.Description, StartTime: publicClock(value.StartTime),
		EndTime: publicClock(value.EndTime), RoomID: value.RoomID, RequiredStaff: value.RequiredStaff,
		Status: value.Status, ActiveGroupID: value.ActiveGroupID, ListKind: value.ListKind,
		IsSpontaneous: value.IsSpontaneous, UnderstaffedAck: value.UnderstaffedAck,
		UnderstaffedNote: value.UnderstaffedNote, CancelReason: value.CancelReason, Notes: value.Notes,
		IdempotencyKey: value.IdempotencyKey, IdempotencyFingerprint: value.IdempotencyFingerprint,
		CreatedBy: value.CreatedBy, StartedBy: value.StartedBy, StartedAt: value.StartedAt,
		CompletedAt: value.CompletedAt, CompletedBy: value.CompletedBy, ReopenUntil: value.ReopenUntil,
		CompletionSnapshot: []byte(value.CompletionSnapshot),
	}
}

func publicActivityInstanceFilter(value scheduleRepo.ActivityInstanceListFilter) timetable.ActivityInstanceFilter {
	return timetable.ActivityInstanceFilter{
		IDs: value.IDs, Date: value.Date, Dates: value.Dates, ActivityGroupIDs: value.ActivityGroupIDs,
		ActiveGroupIDs: value.ActiveGroupIDs, Status: value.Status, IsSpontaneous: value.IsSpontaneous,
		IdempotencyKey: value.IdempotencyKey, Limit: value.Limit, Offset: value.Offset,
	}
}

func publicOptionalInstanceDate(value *scheduleRepo.ActivityInstanceDate) *string {
	if value == nil {
		return nil
	}
	text := value.String()
	return &text
}

func legacyActivityInstanceError(operation string, err error) error {
	if errors.Is(err, timetable.ErrActivityInstanceNotFound) {
		return scheduleRepo.WrapNotFoundDatabaseError(operation)
	}
	return scheduleRepo.WrapDatabaseError(operation, err)
}
