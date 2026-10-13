package services

import (
	"context"
	"errors"

	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// The retained models/schedule Dienstplan rows are served by the Timetable
// composition over the Workforce capability (#2689); the bindings below hand
// it the capability in that composition's vocabulary.

// The two vocabularies spell the same order columns; a drift must not
// compile: a false comparison repeats the false key of this map literal.
var _ = map[bool]struct{}{
	false: {},
	string(workforce.StaffShiftOrderDate) == string(timetableCompose.WorkforceShiftOrderDate) &&
		string(workforce.StaffShiftOrderStaffID) == string(timetableCompose.WorkforceShiftOrderStaffID) &&
		string(workforce.StaffShiftOrderStartTime) == string(timetableCompose.WorkforceShiftOrderStartTime): {},
}

// NewWorkforceShiftRows binds the staff-shift rows to the Workforce capability.
func NewWorkforceShiftRows(capability workforce.Capability) *timetableCompose.WorkforceShiftRows {
	if capability == nil {
		panic("staff shift rows: Workforce capability is required")
	}
	return timetableCompose.NewWorkforceShiftRows(workforceShiftRecords{capability: capability}, peopleCompose.RetainedRecordErrors{})
}

// NewWorkforceShiftTypeRows binds the shift-type rows to the Workforce capability.
func NewWorkforceShiftTypeRows(capability workforce.Capability) *timetableCompose.WorkforceShiftTypeRows {
	if capability == nil {
		panic("shift type rows: Workforce capability is required")
	}
	return timetableCompose.NewWorkforceShiftTypeRows(workforceShiftRecords{capability: capability}, peopleCompose.RetainedRecordErrors{})
}

// workforceShiftRecords translates between the Timetable composition's
// vocabulary and Workforce's.
type workforceShiftRecords struct{ capability workforce.Capability }

// workforceShiftError marks Workforce's answers for the rows.
func workforceShiftError(err error) error {
	switch {
	case errors.Is(err, workforce.ErrStaffShiftNotFound):
		return timetableCompose.MarkRecordError(timetableCompose.ErrWorkforceShiftNotFound, err)
	case errors.Is(err, workforce.ErrShiftTypeNotFound):
		return timetableCompose.MarkRecordError(timetableCompose.ErrWorkforceShiftTypeNotFound, err)
	case errors.Is(err, workforce.ErrInvalidStaffShift), errors.Is(err, workforce.ErrInvalidShiftSeries),
		errors.Is(err, workforce.ErrInvalidShiftType):
		return timetableCompose.MarkRecordError(timetableCompose.ErrWorkforceShiftInvalid, err)
	default:
		return err
	}
}

func workforceShifts(values []workforce.StaffShift) []timetableCompose.WorkforceShift {
	if values == nil {
		return nil
	}
	result := make([]timetableCompose.WorkforceShift, 0, len(values))
	for _, value := range values {
		result = append(result, timetableCompose.WorkforceShift(value))
	}
	return result
}

func (r workforceShiftRecords) CreateStaffShift(ctx context.Context, shift timetableCompose.WorkforceShift) (timetableCompose.WorkforceShift, error) {
	created, err := r.capability.CreateStaffShift(ctx, workforce.StaffShift(shift))
	return timetableCompose.WorkforceShift(created), workforceShiftError(err)
}

func (r workforceShiftRecords) CreateStaffShifts(ctx context.Context, shifts []timetableCompose.WorkforceShift) ([]timetableCompose.WorkforceShift, error) {
	values := make([]workforce.StaffShift, 0, len(shifts))
	for _, shift := range shifts {
		values = append(values, workforce.StaffShift(shift))
	}
	created, err := r.capability.CreateStaffShifts(ctx, values)
	return workforceShifts(created), workforceShiftError(err)
}

func (r workforceShiftRecords) FindStaffShift(ctx context.Context, id int64) (timetableCompose.WorkforceShift, error) {
	value, err := r.capability.FindStaffShift(ctx, id)
	return timetableCompose.WorkforceShift(value), workforceShiftError(err)
}

func (r workforceShiftRecords) UpdateStaffShift(ctx context.Context, shift timetableCompose.WorkforceShift) (timetableCompose.WorkforceShift, error) {
	updated, err := r.capability.UpdateStaffShift(ctx, workforce.StaffShift(shift))
	return timetableCompose.WorkforceShift(updated), workforceShiftError(err)
}

func (r workforceShiftRecords) DeleteStaffShift(ctx context.Context, id int64) error {
	return workforceShiftError(r.capability.DeleteStaffShift(ctx, id))
}

func (r workforceShiftRecords) ListStaffShifts(ctx context.Context, filter timetableCompose.WorkforceShiftFilter) ([]timetableCompose.WorkforceShift, error) {
	order := make([]workforce.StaffShiftOrder, 0, len(filter.Order))
	for _, value := range filter.Order {
		order = append(order, workforce.StaffShiftOrder{Field: workforce.StaffShiftOrderField(value.Field), Descending: value.Descending})
	}
	values, err := r.capability.ListStaffShifts(ctx, workforce.StaffShiftFilter{
		StaffID: filter.StaffID, StaffIDs: filter.StaffIDs, From: filter.From, To: filter.To, Dates: filter.Dates,
		SeriesID: filter.SeriesID, OriginShiftID: filter.OriginShiftID, OriginShiftIDs: filter.OriginShiftIDs,
		SickAbsenceID: filter.SickAbsenceID, Cancelled: filter.Cancelled, Detached: filter.Detached,
		Order: order, Limit: filter.Limit, Offset: filter.Offset,
	})
	return workforceShifts(values), workforceShiftError(err)
}

func (r workforceShiftRecords) UsedStaffShiftWeeks(ctx context.Context, from, to string) ([]string, error) {
	weeks, err := r.capability.UsedStaffShiftWeeks(ctx, from, to)
	return weeks, workforceShiftError(err)
}

func (r workforceShiftRecords) DeleteRegenerableSeriesShifts(ctx context.Context, seriesID int64, from string) (int64, error) {
	deleted, err := r.capability.DeleteRegenerableSeriesShifts(ctx, seriesID, from)
	return deleted, workforceShiftError(err)
}

func (r workforceShiftRecords) RepointDetachedSeriesShifts(ctx context.Context, fromSeriesID, toSeriesID int64, from string) (int64, error) {
	moved, err := r.capability.RepointDetachedSeriesShifts(ctx, fromSeriesID, toSeriesID, from)
	return moved, workforceShiftError(err)
}

func (r workforceShiftRecords) CreateShiftType(ctx context.Context, shiftType timetableCompose.WorkforceShiftType) (timetableCompose.WorkforceShiftType, error) {
	created, err := r.capability.CreateShiftType(ctx, workforce.ShiftType(shiftType))
	return timetableCompose.WorkforceShiftType(created), workforceShiftError(err)
}

func (r workforceShiftRecords) CreateShiftTypeIfAbsent(ctx context.Context, shiftType timetableCompose.WorkforceShiftType) (timetableCompose.WorkforceShiftType, bool, error) {
	created, inserted, err := r.capability.CreateShiftTypeIfAbsent(ctx, workforce.ShiftType(shiftType))
	return timetableCompose.WorkforceShiftType(created), inserted, workforceShiftError(err)
}

func (r workforceShiftRecords) FindShiftType(ctx context.Context, id int64) (timetableCompose.WorkforceShiftType, error) {
	value, err := r.capability.FindShiftType(ctx, id)
	return timetableCompose.WorkforceShiftType(value), workforceShiftError(err)
}

func (r workforceShiftRecords) UpdateShiftType(ctx context.Context, shiftType timetableCompose.WorkforceShiftType) (timetableCompose.WorkforceShiftType, error) {
	updated, err := r.capability.UpdateShiftType(ctx, workforce.ShiftType(shiftType))
	return timetableCompose.WorkforceShiftType(updated), workforceShiftError(err)
}

func (r workforceShiftRecords) DeleteShiftType(ctx context.Context, id int64) error {
	return workforceShiftError(r.capability.DeleteShiftType(ctx, id))
}

func (r workforceShiftRecords) ListShiftTypes(ctx context.Context) ([]timetableCompose.WorkforceShiftType, error) {
	values, err := r.capability.ListShiftTypes(ctx)
	if values == nil {
		return nil, workforceShiftError(err)
	}
	result := make([]timetableCompose.WorkforceShiftType, 0, len(values))
	for _, value := range values {
		result = append(result, timetableCompose.WorkforceShiftType(value))
	}
	return result, workforceShiftError(err)
}
