//go:build testhooks

package fixturetest

import (
	context "context"

	records "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	ports "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type ConflictShifts interface {
	FindByDateRange(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) ([]*StaffShift, error)
	FindByStaffIDsAndDates(arg0 context.Context, arg1 []int64, arg2 []calendar.Date) ([]*StaffShift, error)
	FindUsedCalendarWeeks(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) ([]calendar.Date, error)
}

type intoConflictShifts struct {
	value ConflictShifts
}

func IntoConflictShifts(value ConflictShifts) ports.ConflictShifts {
	if original, ok := value.(interface{ nativeRecordSource() any }); ok {
		if capability, ok := original.nativeRecordSource().(ports.ConflictShifts); ok {
			return capability
		}
	}
	if original, ok := value.(interface{ nativeConflictShifts() ports.ConflictShifts }); ok {
		return original.nativeConflictShifts()
	}
	if value == nil {
		return nil
	}
	return intoConflictShifts{value: value}
}

func (adapter intoConflictShifts) FindByDateRange(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) ([]*records.StaffShift, error) {
	result0, result1 := adapter.value.FindByDateRange(arg0, arg1, arg2)
	return toNative10(result0), result1
}

func (adapter intoConflictShifts) FindByStaffIDsAndDates(arg0 context.Context, arg1 []int64, arg2 []calendar.Date) ([]*records.StaffShift, error) {
	result0, result1 := adapter.value.FindByStaffIDsAndDates(arg0, arg1, arg2)
	return toNative10(result0), result1
}

func (adapter intoConflictShifts) FindUsedCalendarWeeks(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) ([]calendar.Date, error) {
	result0, result1 := adapter.value.FindUsedCalendarWeeks(arg0, arg1, arg2)
	return result0, result1
}
