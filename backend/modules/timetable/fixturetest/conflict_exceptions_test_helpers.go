//go:build testhooks

package fixturetest

import (
	context "context"

	records "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	ports "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type ConflictExceptions interface {
	FindByActivityGroupAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*ActivityException, error)
	FindByDateRange(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) ([]*ActivityException, error)
}

type intoConflictExceptions struct {
	value ConflictExceptions
}

func IntoConflictExceptions(value ConflictExceptions) ports.ConflictExceptions {
	if original, ok := value.(interface{ nativeRecordSource() any }); ok {
		if capability, ok := original.nativeRecordSource().(ports.ConflictExceptions); ok {
			return capability
		}
	}
	if original, ok := value.(interface {
		nativeConflictExceptions() ports.ConflictExceptions
	}); ok {
		return original.nativeConflictExceptions()
	}
	if value == nil {
		return nil
	}
	return intoConflictExceptions{value: value}
}

func (adapter intoConflictExceptions) FindByActivityGroupAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*records.ActivityException, error) {
	result0, result1 := adapter.value.FindByActivityGroupAndDateRange(arg0, arg1, arg2, arg3)
	return toNative1(result0), result1
}

func (adapter intoConflictExceptions) FindByDateRange(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) ([]*records.ActivityException, error) {
	result0, result1 := adapter.value.FindByDateRange(arg0, arg1, arg2)
	return toNative1(result0), result1
}
