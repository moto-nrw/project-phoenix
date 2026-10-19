//go:build testhooks

package fixturetest

import (
	context "context"

	records "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	ports "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type ConflictInstances interface {
	FindByActivityGroupAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*ActivityInstance, error)
	FindByID(arg0 context.Context, arg1 int64) (*ActivityInstance, error)
	FindByTenantAndDateRange(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) ([]*ActivityInstance, error)
}

type intoConflictInstances struct {
	value ConflictInstances
}

func IntoConflictInstances(value ConflictInstances) ports.ConflictInstances {
	if original, ok := value.(interface{ nativeRecordSource() any }); ok {
		if capability, ok := original.nativeRecordSource().(ports.ConflictInstances); ok {
			return capability
		}
	}
	if original, ok := value.(interface {
		nativeConflictInstances() ports.ConflictInstances
	}); ok {
		return original.nativeConflictInstances()
	}
	if value == nil {
		return nil
	}
	return intoConflictInstances{value: value}
}

func (adapter intoConflictInstances) FindByActivityGroupAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*records.ActivityInstance, error) {
	result0, result1 := adapter.value.FindByActivityGroupAndDateRange(arg0, arg1, arg2, arg3)
	return toNative3(result0), result1
}

func (adapter intoConflictInstances) FindByID(arg0 context.Context, arg1 int64) (*records.ActivityInstance, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return toNative2(result0), result1
}

func (adapter intoConflictInstances) FindByTenantAndDateRange(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) ([]*records.ActivityInstance, error) {
	result0, result1 := adapter.value.FindByTenantAndDateRange(arg0, arg1, arg2)
	return toNative3(result0), result1
}
