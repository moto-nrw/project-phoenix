//go:build testhooks

package fixturetest

import (
	context "context"

	ports "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type WorkforceShiftRows interface {
	Create(arg0 context.Context, arg1 *StaffShift) error
	Delete(arg0 context.Context, arg1 int64) error
	FindByDateRange(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) ([]*StaffShift, error)
	FindByID(arg0 context.Context, arg1 int64) (*StaffShift, error)
	FindByStaffAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*StaffShift, error)
	FindByStaffIDsAndDates(arg0 context.Context, arg1 []int64, arg2 []calendar.Date) ([]*StaffShift, error)
	FindUsedCalendarWeeks(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) ([]calendar.Date, error)
	Update(arg0 context.Context, arg1 *StaffShift) error
}

type fromWorkforceShiftRows struct {
	value ports.WorkforceShiftRows
}

func FromWorkforceShiftRows(value ports.WorkforceShiftRows) WorkforceShiftRows {
	if original, ok := value.(interface{ fixtureWorkforceShiftRows() WorkforceShiftRows }); ok {
		return original.fixtureWorkforceShiftRows()
	}
	if value == nil {
		return nil
	}
	return fromWorkforceShiftRows{value: value}
}

func (adapter fromWorkforceShiftRows) Create(arg0 context.Context, arg1 *StaffShift) error {
	result0 := adapter.value.Create(arg0, toNative45(arg1))
	return result0
}

func (adapter fromWorkforceShiftRows) Delete(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.Delete(arg0, arg1)
	return result0
}

func (adapter fromWorkforceShiftRows) FindByDateRange(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) ([]*StaffShift, error) {
	result0, result1 := adapter.value.FindByDateRange(arg0, arg1, arg2)
	return fromNative10(result0), result1
}

func (adapter fromWorkforceShiftRows) FindByID(arg0 context.Context, arg1 int64) (*StaffShift, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return fromNative45(result0), result1
}

func (adapter fromWorkforceShiftRows) FindByStaffAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*StaffShift, error) {
	result0, result1 := adapter.value.FindByStaffAndDateRange(arg0, arg1, arg2, arg3)
	return fromNative10(result0), result1
}

func (adapter fromWorkforceShiftRows) FindByStaffIDsAndDates(arg0 context.Context, arg1 []int64, arg2 []calendar.Date) ([]*StaffShift, error) {
	result0, result1 := adapter.value.FindByStaffIDsAndDates(arg0, arg1, arg2)
	return fromNative10(result0), result1
}

func (adapter fromWorkforceShiftRows) FindUsedCalendarWeeks(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) ([]calendar.Date, error) {
	result0, result1 := adapter.value.FindUsedCalendarWeeks(arg0, arg1, arg2)
	return result0, result1
}

func (adapter fromWorkforceShiftRows) Update(arg0 context.Context, arg1 *StaffShift) error {
	result0 := adapter.value.Update(arg0, toNative45(arg1))
	return result0
}

func (adapter fromWorkforceShiftRows) nativeRecordSource() any { return adapter.value }
