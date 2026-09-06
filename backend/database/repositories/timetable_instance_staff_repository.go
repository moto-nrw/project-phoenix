package repositories

import (
	"context"
	"errors"
	"fmt"

	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

type timetableInstanceStaffRepository struct {
	timetable timetable.InstanceStaffCapability
}

func (r timetableInstanceStaffRepository) Create(ctx context.Context, value *scheduleModels.InstanceStaff) error {
	if value == nil {
		return errors.New("instance staff cannot be nil or zero value")
	}
	if err := value.Validate(); err != nil {
		return err
	}
	created, err := r.timetable.CreateInstanceStaff(ctx, publicInstanceStaffInput(value))
	if err != nil {
		return scheduleRepo.WrapDatabaseError("create", err)
	}
	replaceLegacyInstanceStaff(value, created)
	return nil
}

func (r timetableInstanceStaffRepository) FindByID(ctx context.Context, id any) (*scheduleModels.InstanceStaff, error) {
	rowID, ok := legacyGroupID(id)
	if !ok {
		return nil, scheduleRepo.WrapDatabaseError("find by id", fmt.Errorf("invalid instance staff id %T", id))
	}
	value, err := r.timetable.FindInstanceStaff(ctx, rowID)
	if err != nil {
		return nil, legacyInstanceStaffError("find by id", err)
	}
	return legacyInstanceStaff(value), nil
}

func (r timetableInstanceStaffRepository) Update(ctx context.Context, value *scheduleModels.InstanceStaff) error {
	if value == nil {
		return errors.New("instance staff cannot be nil or zero value")
	}
	if err := value.Validate(); err != nil {
		return err
	}
	updated, err := r.timetable.UpdateInstanceStaff(ctx, value.ID, publicInstanceStaffInput(value))
	if err != nil {
		return legacyInstanceStaffError("update", err)
	}
	replaceLegacyInstanceStaff(value, updated)
	return nil
}

func (r timetableInstanceStaffRepository) Delete(ctx context.Context, id any) error {
	rowID, ok := legacyGroupID(id)
	if !ok {
		return scheduleRepo.WrapDatabaseError("delete", fmt.Errorf("invalid instance staff id %T", id))
	}
	if err := r.timetable.DeleteInstanceStaff(ctx, rowID); err != nil {
		return scheduleRepo.WrapDatabaseError("delete", err)
	}
	return nil
}

func (r timetableInstanceStaffRepository) List(ctx context.Context, options *scheduleRepo.InstanceStaffQueryOptions) ([]*scheduleModels.InstanceStaff, error) {
	filter, err := scheduleRepo.InstanceStaffListOptions(options)
	if err != nil {
		return nil, scheduleRepo.WrapDatabaseError("list with options", err)
	}
	return r.list(ctx, timetable.InstanceStaffFilter{
		IDs: filter.IDs, InstanceIDs: filter.InstanceIDs, StaffIDs: filter.StaffIDs,
		SickAbsenceID: filter.SickAbsenceID, Limit: filter.Limit, Offset: filter.Offset,
	}, "list with options")
}

func (r timetableInstanceStaffRepository) UpdateColumns(ctx context.Context, value *scheduleModels.InstanceStaff, columns ...string) (int64, error) {
	if value == nil {
		return 0, errors.New("InstanceStaff cannot be nil or zero value")
	}
	rows, err := r.timetable.PatchInstanceStaff(ctx, value.ID, publicInstanceStaffInput(value), columns)
	if err != nil {
		return 0, scheduleRepo.WrapDatabaseError("update columns", err)
	}
	return rows, nil
}

func (r timetableInstanceStaffRepository) FindByInstanceID(ctx context.Context, instanceID int64) ([]*scheduleModels.InstanceStaff, error) {
	return r.list(ctx, timetable.InstanceStaffFilter{InstanceIDs: []int64{instanceID}, OrderByCreated: true}, "find by instance id")
}

func (r timetableInstanceStaffRepository) FindByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*scheduleModels.InstanceStaff, error) {
	if len(instanceIDs) == 0 {
		return []*scheduleModels.InstanceStaff{}, nil
	}
	return r.list(ctx, timetable.InstanceStaffFilter{InstanceIDs: instanceIDs, OrderByInstanceAndCreated: true}, "find by instance ids")
}

func (r timetableInstanceStaffRepository) FindByStaffAndDate(ctx context.Context, staffID int64, date scheduleRepo.InstanceStaffDate) ([]*scheduleModels.InstanceStaff, error) {
	text := date.String()
	return r.list(ctx, timetable.InstanceStaffFilter{StaffIDs: []int64{staffID}, Date: &text, OrderByActivityTime: true}, "find by staff and date")
}

func (r timetableInstanceStaffRepository) FindByStaffIDsAndDate(ctx context.Context, staffIDs []int64, date scheduleRepo.InstanceStaffDate) ([]*scheduleModels.InstanceStaff, error) {
	if len(staffIDs) == 0 {
		return []*scheduleModels.InstanceStaff{}, nil
	}
	text := date.String()
	return r.list(ctx, timetable.InstanceStaffFilter{StaffIDs: staffIDs, Date: &text, OrderByActivityTime: true}, "find by staff ids and date")
}

func (r timetableInstanceStaffRepository) FindByStaffAndDateRange(ctx context.Context, staffID int64, from, to scheduleRepo.InstanceStaffDate) ([]*scheduleModels.InstanceStaff, error) {
	fromText, toText := from.String(), to.String()
	return r.list(ctx, timetable.InstanceStaffFilter{
		StaffIDs: []int64{staffID}, FromDate: &fromText, ToDate: &toText, OrderByActivityDateTime: true,
	}, "find by staff and date range")
}

func (r timetableInstanceStaffRepository) CountNonAbsentByInstanceIDs(ctx context.Context, instanceIDs []int64) (map[int64]int, error) {
	result, err := r.timetable.CountNonAbsentInstanceStaff(ctx, instanceIDs)
	if err != nil {
		return nil, scheduleRepo.WrapDatabaseError("count non-absent by instance ids", err)
	}
	return result, nil
}

func (r timetableInstanceStaffRepository) DeleteByInstanceID(ctx context.Context, instanceID int64) error {
	if err := r.timetable.DeleteInstanceStaffByInstance(ctx, instanceID); err != nil {
		return scheduleRepo.WrapDatabaseError("delete by instance id", err)
	}
	return nil
}

func (r timetableInstanceStaffRepository) DeleteUpcomingByStaffID(ctx context.Context, staffID int64, after scheduleRepo.InstanceStaffDate) (int64, error) {
	rows, err := r.timetable.DeleteUpcomingInstanceStaff(ctx, staffID, after.String())
	if err != nil {
		return 0, scheduleRepo.WrapDatabaseError("delete upcoming by staff id", err)
	}
	return rows, nil
}

func (r timetableInstanceStaffRepository) list(ctx context.Context, filter timetable.InstanceStaffFilter, operation string) ([]*scheduleModels.InstanceStaff, error) {
	values, err := r.timetable.ListInstanceStaff(ctx, filter)
	if err != nil {
		return nil, scheduleRepo.WrapDatabaseError(operation, err)
	}
	result := make([]*scheduleModels.InstanceStaff, 0, len(values))
	for _, value := range values {
		result = append(result, legacyInstanceStaff(value))
	}
	return result, nil
}

func legacyInstanceStaff(value timetable.InstanceStaff) *scheduleModels.InstanceStaff {
	result := &scheduleModels.InstanceStaff{}
	replaceLegacyInstanceStaff(result, value)
	return result
}

func replaceLegacyInstanceStaff(result *scheduleModels.InstanceStaff, value timetable.InstanceStaff) {
	*result = scheduleModels.InstanceStaff{
		InstanceID: value.InstanceID, StaffID: value.StaffID, RoomID: value.RoomID,
		IsPrimary: value.IsPrimary, IsSubstitute: value.IsSubstitute, IsAbsent: value.IsAbsent,
		AbsenceReason: value.AbsenceReason, SickAbsenceID: value.SickAbsenceID,
	}
	result.ID, result.CreatedAt, result.UpdatedAt = value.ID, value.CreatedAt, value.UpdatedAt
	result.SetTenantID(value.TenantID)
}

func publicInstanceStaffInput(value *scheduleModels.InstanceStaff) timetable.InstanceStaffInput {
	return timetable.InstanceStaffInput{
		InstanceID: value.InstanceID, StaffID: value.StaffID, RoomID: value.RoomID,
		IsPrimary: value.IsPrimary, IsSubstitute: value.IsSubstitute, IsAbsent: value.IsAbsent,
		AbsenceReason: value.AbsenceReason, SickAbsenceID: value.SickAbsenceID,
	}
}

func legacyInstanceStaffError(operation string, err error) error {
	if errors.Is(err, timetable.ErrInstanceStaffNotFound) {
		return scheduleRepo.WrapNotFoundDatabaseError(operation)
	}
	return scheduleRepo.WrapDatabaseError(operation, err)
}
