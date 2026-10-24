//go:build testhooks

package fixturetest

import (
	context "context"

	records "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	ports "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type ConflictArrivalExceptions interface {
	FindByStudentIDsAndDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]*StudentArrivalException, error)
}

type intoConflictArrivalExceptions struct {
	value ConflictArrivalExceptions
}

func IntoConflictArrivalExceptions(value ConflictArrivalExceptions) ports.ConflictArrivalExceptions {
	if original, ok := value.(interface{ nativeRecordSource() any }); ok {
		if capability, ok := original.nativeRecordSource().(ports.ConflictArrivalExceptions); ok {
			return capability
		}
	}
	if value == nil {
		return nil
	}
	return intoConflictArrivalExceptions{value: value}
}

func (adapter intoConflictArrivalExceptions) FindByStudentIDsAndDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]*records.StudentArrivalException, error) {
	result0, result1 := adapter.value.FindByStudentIDsAndDate(arg0, arg1, arg2)
	return toNative13(result0), result1
}
