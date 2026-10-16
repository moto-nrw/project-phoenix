package application

import (
	"context"
	"errors"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
)

// The adapters in this file serve the retained models/schedule Dienstplan
// row types over the Workforce capability (#2689) to the retained consumers
// that still speak them: the Timetable coverage probe, the staff calendar
// feed, the plan export and the legacy suites. They perform no persistence of
// their own: every adapter maps the row models onto the public capability
// types and preserves the error shapes those callers still classify on. The
// Dienstplan services themselves speak Workforce's own rows since #3424.

const shiftWallClockLayout = "15:04:05"

// WorkforceShiftRows serves the concrete schedule.staff_shifts rows.
type WorkforceShiftRows struct {
	workforce WorkforceShiftRecords
	errs      RetainedRecordErrors
}

// NewWorkforceShiftRows binds the staff-shift rows to the Workforce records.
func NewWorkforceShiftRows(records WorkforceShiftRecords, errs RetainedRecordErrors) *WorkforceShiftRows {
	return &WorkforceShiftRows{workforce: records, errs: errs}
}

func (r *WorkforceShiftRows) Create(ctx context.Context, shift *scheduleModels.StaffShift) error {
	if shift == nil {
		return errors.New("StaffShift cannot be nil or zero value")
	}
	if err := shift.Validate(); err != nil {
		return err
	}
	created, err := r.workforce.CreateStaffShift(ctx, shiftToCapability(shift))
	if err != nil {
		return shiftWriteError(r.errs, "create", err)
	}
	applyShiftToRow(shift, created)
	return nil
}

func (r *WorkforceShiftRows) FindByID(ctx context.Context, id any) (*scheduleModels.StaffShift, error) {
	shiftID, err := shiftRowID(id)
	if err != nil {
		return nil, r.errs.WrapRecordError("find by id", err)
	}
	value, err := r.workforce.FindStaffShift(ctx, shiftID)
	if err != nil {
		return nil, shiftReadError(r.errs, "find by id", err, ErrWorkforceShiftNotFound)
	}
	return shiftToRow(value), nil
}

func (r *WorkforceShiftRows) Update(ctx context.Context, shift *scheduleModels.StaffShift) error {
	if shift == nil {
		return errors.New("StaffShift cannot be nil or zero value")
	}
	if err := shift.Validate(); err != nil {
		return err
	}
	updated, err := r.workforce.UpdateStaffShift(ctx, shiftToCapability(shift))
	if err != nil {
		if errors.Is(err, ErrWorkforceShiftNotFound) {
			return shiftRowsAffectedError(r.errs, "update StaffShift")
		}
		return shiftWriteError(r.errs, "update", err)
	}
	applyShiftToRow(shift, updated)
	return nil
}

func (r *WorkforceShiftRows) Delete(ctx context.Context, id any) error {
	shiftID, err := shiftRowID(id)
	if err != nil {
		return r.errs.WrapRecordError("delete", err)
	}
	if err := r.workforce.DeleteStaffShift(ctx, shiftID); err != nil {
		return shiftWriteError(r.errs, "delete", err)
	}
	return nil
}

func (r *WorkforceShiftRows) FindByDateRange(ctx context.Context, start, end scheduleModels.Date) ([]*scheduleModels.StaffShift, error) {
	return r.list(ctx, "find staff shifts by date range", WorkforceShiftFilter{
		From: start.String(), To: end.String(),
		Order: shiftOrder(WorkforceShiftOrderDate, WorkforceShiftOrderStaffID, WorkforceShiftOrderStartTime),
	})
}

func (r *WorkforceShiftRows) FindByStaffAndDateRange(ctx context.Context, staffID int64, start, end scheduleModels.Date) ([]*scheduleModels.StaffShift, error) {
	return r.list(ctx, "find staff shifts by staff and date range", WorkforceShiftFilter{
		StaffID: staffID, From: start.String(), To: end.String(),
		Order: shiftOrder(WorkforceShiftOrderDate, WorkforceShiftOrderStartTime),
	})
}

func (r *WorkforceShiftRows) FindByStaffIDsAndDates(ctx context.Context, staffIDs []int64, dates []scheduleModels.Date) ([]*scheduleModels.StaffShift, error) {
	if len(staffIDs) == 0 || len(dates) == 0 {
		return nil, nil
	}
	days := make([]string, 0, len(dates))
	for _, date := range dates {
		days = append(days, date.String())
	}
	return r.list(ctx, "find staff shifts by staff IDs and dates", WorkforceShiftFilter{
		StaffIDs: staffIDs, Dates: days,
		Order: shiftOrder(WorkforceShiftOrderDate, WorkforceShiftOrderStaffID, WorkforceShiftOrderStartTime),
	})
}

func (r *WorkforceShiftRows) FindUsedCalendarWeeks(ctx context.Context, start, end scheduleModels.Date) ([]scheduleModels.Date, error) {
	weeks, err := r.workforce.UsedStaffShiftWeeks(ctx, start.String(), end.String())
	if err != nil {
		return nil, shiftReadError(r.errs, "find used staff-shift calendar weeks", err, nil)
	}
	result := make([]scheduleModels.Date, 0, len(weeks))
	for _, week := range weeks {
		result = append(result, scheduleModels.Date(week))
	}
	return result, nil
}

func (r *WorkforceShiftRows) list(ctx context.Context, op string, filter WorkforceShiftFilter) ([]*scheduleModels.StaffShift, error) {
	values, err := r.workforce.ListStaffShifts(ctx, filter)
	if err != nil {
		return nil, shiftReadError(r.errs, op, err, nil)
	}
	// Empty, not nil: callers serialize the result straight to JSON.
	result := make([]*scheduleModels.StaffShift, 0, len(values))
	for _, value := range values {
		result = append(result, shiftToRow(value))
	}
	return result, nil
}

func shiftOrder(fields ...WorkforceShiftOrderField) []WorkforceShiftOrder {
	order := make([]WorkforceShiftOrder, 0, len(fields))
	for _, field := range fields {
		order = append(order, WorkforceShiftOrder{Field: field})
	}
	return order
}
