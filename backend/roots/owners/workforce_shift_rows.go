package owners

import (
	"context"
	"errors"

	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
)

// newWorkforceShiftRows binds the staff-shift rows to the Workforce capability.
func newWorkforceShiftRows(capability workforceModule.Capability) timetablePorts.WorkforceShiftRows {
	if capability == nil {
		panic("staff shift rows: Workforce capability is required")
	}
	return timetableCompose.NewWorkforceShiftRows(workforceShiftRecords{capability: capability}, transactionruntime.RecordErrors{})
}

// newWorkforceShiftTypeRows binds the shift-type rows to the Workforce capability.
func newWorkforceShiftTypeRows(capability workforceModule.Capability) timetablePorts.WorkforceShiftTypeRows {
	if capability == nil {
		panic("shift type rows: Workforce capability is required")
	}
	return timetableCompose.NewWorkforceShiftTypeRows(workforceShiftRecords{capability: capability}, transactionruntime.RecordErrors{})
}

// workforceShiftRecords translates between the Timetable composition's
// vocabulary and Workforce's.
type workforceShiftRecords struct{ capability workforceModule.Capability }

// workforceShiftError marks Workforce's answers for the rows.
func workforceShiftError(err error) error {
	switch {
	case errors.Is(err, workforceModule.ErrStaffShiftNotFound):
		return timetablePorts.MarkRecordError(timetablePorts.ErrWorkforceShiftNotFound, err)
	case errors.Is(err, workforceModule.ErrInvalidStaffShift), errors.Is(err, workforceModule.ErrInvalidShiftSeries),
		errors.Is(err, workforceModule.ErrInvalidShiftType):
		return timetablePorts.MarkRecordError(timetablePorts.ErrWorkforceShiftInvalid, err)
	default:
		return err
	}
}

func workforceShifts(values []workforceModule.StaffShift) []timetablePorts.WorkforceShift {
	if values == nil {
		return nil
	}
	result := make([]timetablePorts.WorkforceShift, 0, len(values))
	for _, value := range values {
		result = append(result, timetablePorts.WorkforceShift(value))
	}
	return result
}

func (r workforceShiftRecords) CreateStaffShift(ctx context.Context, shift timetablePorts.WorkforceShift) (timetablePorts.WorkforceShift, error) {
	created, err := r.capability.CreateStaffShift(ctx, workforceModule.StaffShift(shift))
	return timetablePorts.WorkforceShift(created), workforceShiftError(err)
}

func (r workforceShiftRecords) FindStaffShift(ctx context.Context, id int64) (timetablePorts.WorkforceShift, error) {
	value, err := r.capability.FindStaffShift(ctx, id)
	return timetablePorts.WorkforceShift(value), workforceShiftError(err)
}

func (r workforceShiftRecords) UpdateStaffShift(ctx context.Context, shift timetablePorts.WorkforceShift) (timetablePorts.WorkforceShift, error) {
	updated, err := r.capability.UpdateStaffShift(ctx, workforceModule.StaffShift(shift))
	return timetablePorts.WorkforceShift(updated), workforceShiftError(err)
}

func (r workforceShiftRecords) DeleteStaffShift(ctx context.Context, id int64) error {
	return workforceShiftError(r.capability.DeleteStaffShift(ctx, id))
}

func (r workforceShiftRecords) ListStaffShifts(ctx context.Context, filter timetablePorts.WorkforceShiftFilter) ([]timetablePorts.WorkforceShift, error) {
	order := make([]workforceModule.StaffShiftOrder, 0, len(filter.Order))
	for _, value := range filter.Order {
		order = append(order, workforceModule.StaffShiftOrder{Field: workforceModule.StaffShiftOrderField(value.Field)})
	}
	values, err := r.capability.ListStaffShifts(ctx, workforceModule.StaffShiftFilter{
		StaffID: filter.StaffID, StaffIDs: filter.StaffIDs, From: filter.From, To: filter.To, Dates: filter.Dates,
		Order: order,
	})
	return workforceShifts(values), workforceShiftError(err)
}

func (r workforceShiftRecords) UsedStaffShiftWeeks(ctx context.Context, from, to string) ([]string, error) {
	weeks, err := r.capability.UsedStaffShiftWeeks(ctx, from, to)
	return weeks, workforceShiftError(err)
}

func (r workforceShiftRecords) CreateShiftType(ctx context.Context, shiftType timetablePorts.WorkforceShiftType) (timetablePorts.WorkforceShiftType, error) {
	created, err := r.capability.CreateShiftType(ctx, workforceModule.ShiftType(shiftType))
	return timetablePorts.WorkforceShiftType(created), workforceShiftError(err)
}

func (r workforceShiftRecords) DeleteShiftType(ctx context.Context, id int64) error {
	return workforceShiftError(r.capability.DeleteShiftType(ctx, id))
}

func (r workforceShiftRecords) ListShiftTypes(ctx context.Context) ([]timetablePorts.WorkforceShiftType, error) {
	values, err := r.capability.ListShiftTypes(ctx)
	if values == nil {
		return nil, workforceShiftError(err)
	}
	result := make([]timetablePorts.WorkforceShiftType, 0, len(values))
	for _, value := range values {
		result = append(result, timetablePorts.WorkforceShiftType(value))
	}
	return result, workforceShiftError(err)
}
