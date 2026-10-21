package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The instance lifecycle, the materializer, the template writes, the
// operational day and the conflict and coverage reads still compute on the
// legacy instance staff row. They call the owner's instance staff
// capability through these reads and writes, which convert the rows and
// report failures under the operation names the retained repository used.

func createLegacyInstanceStaff(ctx context.Context, staff timetable.InstanceStaffCommand, value *scheduleModels.InstanceStaff) error {
	if value == nil {
		return errors.New("instance staff cannot be nil or zero value")
	}
	if err := value.Validate(); err != nil {
		return err
	}
	created, err := staff.CreateInstanceStaff(ctx, publicInstanceStaffInput(value))
	if err != nil {
		return WrapDatabaseError("create", err)
	}
	replaceLegacyInstanceStaff(value, created)
	return nil
}

func findLegacyInstanceStaff(ctx context.Context, staff timetable.InstanceStaffQuery, id int64) (*scheduleModels.InstanceStaff, error) {
	value, err := staff.FindInstanceStaff(ctx, id)
	if err != nil {
		return nil, legacyInstanceStaffError("find by id", err)
	}
	return legacyInstanceStaff(value), nil
}

func updateLegacyInstanceStaff(ctx context.Context, staff timetable.InstanceStaffCommand, value *scheduleModels.InstanceStaff) error {
	if value == nil {
		return errors.New("instance staff cannot be nil or zero value")
	}
	if err := value.Validate(); err != nil {
		return err
	}
	updated, err := staff.UpdateInstanceStaff(ctx, value.ID, publicInstanceStaffInput(value))
	if err != nil {
		return legacyInstanceStaffError("update", err)
	}
	replaceLegacyInstanceStaff(value, updated)
	return nil
}

func deleteLegacyInstanceStaff(ctx context.Context, staff timetable.InstanceStaffCommand, id int64) error {
	if err := staff.DeleteInstanceStaff(ctx, id); err != nil {
		return WrapDatabaseError("delete", err)
	}
	return nil
}

// patchLegacyInstanceStaff writes the named columns of the row.
func patchLegacyInstanceStaff(ctx context.Context, staff timetable.InstanceStaffCommand, value *scheduleModels.InstanceStaff, columns ...string) (int64, error) {
	if value == nil {
		return 0, errors.New("InstanceStaff cannot be nil or zero value")
	}
	rows, err := staff.PatchInstanceStaff(ctx, value.ID, publicInstanceStaffInput(value), columns)
	if err != nil {
		return 0, WrapDatabaseError("update columns", err)
	}
	return rows, nil
}

func deleteLegacyInstanceStaffByInstance(ctx context.Context, staff timetable.InstanceStaffCommand, instanceID int64) error {
	if err := staff.DeleteInstanceStaffByInstance(ctx, instanceID); err != nil {
		return WrapDatabaseError("delete by instance id", err)
	}
	return nil
}

// staffOfInstance lists an instance's staff in the order they were added.
func staffOfInstance(ctx context.Context, staff timetable.InstanceStaffQuery, instanceID int64) ([]*scheduleModels.InstanceStaff, error) {
	return listLegacyInstanceStaff(ctx, staff, timetable.InstanceStaffFilter{InstanceIDs: []int64{instanceID}, OrderByCreated: true}, "find by instance id")
}

// staffOfInstances lists the staff of several instances; no instances list
// nobody.
func staffOfInstances(ctx context.Context, staff timetable.InstanceStaffQuery, instanceIDs []int64) ([]*scheduleModels.InstanceStaff, error) {
	if len(instanceIDs) == 0 {
		return []*scheduleModels.InstanceStaff{}, nil
	}
	return listLegacyInstanceStaff(ctx, staff, timetable.InstanceStaffFilter{InstanceIDs: instanceIDs, OrderByInstanceAndCreated: true}, "find by instance ids")
}

// assignmentsOfStaffOn lists a staff member's assignments on a date by time.
func assignmentsOfStaffOn(ctx context.Context, staff timetable.InstanceStaffQuery, staffID int64, date calendar.Date) ([]*scheduleModels.InstanceStaff, error) {
	text := date.String()
	return listLegacyInstanceStaff(ctx, staff, timetable.InstanceStaffFilter{StaffIDs: []int64{staffID}, Date: &text, OrderByActivityTime: true}, "find by staff and date")
}

// assignmentsOfStaffMembersOn lists several staff members' assignments on
// a date; no staff members list nothing.
func assignmentsOfStaffMembersOn(ctx context.Context, staff timetable.InstanceStaffQuery, staffIDs []int64, date calendar.Date) ([]*scheduleModels.InstanceStaff, error) {
	if len(staffIDs) == 0 {
		return []*scheduleModels.InstanceStaff{}, nil
	}
	text := date.String()
	return listLegacyInstanceStaff(ctx, staff, timetable.InstanceStaffFilter{StaffIDs: staffIDs, Date: &text, OrderByActivityTime: true}, "find by staff ids and date")
}

// assignmentsOfStaffBetween lists a staff member's assignments in an
// inclusive date range.
func assignmentsOfStaffBetween(ctx context.Context, staff timetable.InstanceStaffQuery, staffID int64, from, to calendar.Date) ([]*scheduleModels.InstanceStaff, error) {
	fromText, toText := from.String(), to.String()
	return listLegacyInstanceStaff(ctx, staff, timetable.InstanceStaffFilter{
		StaffIDs: []int64{staffID}, FromDate: &fromText, ToDate: &toText, OrderByActivityDateTime: true,
	}, "find by staff and date range")
}

func countLegacyNonAbsentInstanceStaff(ctx context.Context, staff timetable.InstanceStaffQuery, instanceIDs []int64) (map[int64]int, error) {
	result, err := staff.CountNonAbsentInstanceStaff(ctx, instanceIDs)
	if err != nil {
		return nil, WrapDatabaseError("count non-absent by instance ids", err)
	}
	return result, nil
}

func listLegacyInstanceStaff(ctx context.Context, staff timetable.InstanceStaffQuery, filter timetable.InstanceStaffFilter, operation string) ([]*scheduleModels.InstanceStaff, error) {
	values, err := staff.ListInstanceStaff(ctx, filter)
	if err != nil {
		return nil, WrapDatabaseError(operation, err)
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
		return WrapNotFoundDatabaseError(operation)
	}
	return WrapDatabaseError(operation, err)
}
