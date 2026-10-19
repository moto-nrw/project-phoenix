//go:build testhooks

package fixturetest

import (
	context "context"

	records "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	ports "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

type ConflictCalendarPeriods interface {
	FindByID(arg0 context.Context, arg1 int64) (*CalendarPeriod, error)
}

type intoConflictCalendarPeriods struct {
	value ConflictCalendarPeriods
}

func IntoConflictCalendarPeriods(value ConflictCalendarPeriods) ports.ConflictCalendarPeriods {
	if original, ok := value.(interface{ nativeRecordSource() any }); ok {
		if capability, ok := original.nativeRecordSource().(ports.ConflictCalendarPeriods); ok {
			return capability
		}
	}
	if original, ok := value.(interface {
		nativeConflictCalendarPeriods() ports.ConflictCalendarPeriods
	}); ok {
		return original.nativeConflictCalendarPeriods()
	}
	if value == nil {
		return nil
	}
	return intoConflictCalendarPeriods{value: value}
}

func (adapter intoConflictCalendarPeriods) FindByID(arg0 context.Context, arg1 int64) (*records.CalendarPeriod, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return toNative7(result0), result1
}
