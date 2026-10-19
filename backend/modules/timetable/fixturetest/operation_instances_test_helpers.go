//go:build testhooks

package fixturetest

import (
	context "context"

	records "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	ports "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type OperationInstances interface {
	FindByActiveGroupID(arg0 context.Context, arg1 int64) (*ActivityInstance, error)
	FindByID(arg0 context.Context, arg1 int64) (*ActivityInstance, error)
	FindByTenantAndDate(arg0 context.Context, arg1 calendar.Date) ([]*ActivityInstance, error)
}

type intoOperationInstances struct {
	value OperationInstances
}

func IntoOperationInstances(value OperationInstances) ports.OperationInstances {
	if original, ok := value.(interface{ nativeRecordSource() any }); ok {
		if capability, ok := original.nativeRecordSource().(ports.OperationInstances); ok {
			return capability
		}
	}
	if original, ok := value.(interface {
		nativeOperationInstances() ports.OperationInstances
	}); ok {
		return original.nativeOperationInstances()
	}
	if value == nil {
		return nil
	}
	return intoOperationInstances{value: value}
}

func (adapter intoOperationInstances) FindByActiveGroupID(arg0 context.Context, arg1 int64) (*records.ActivityInstance, error) {
	result0, result1 := adapter.value.FindByActiveGroupID(arg0, arg1)
	return toNative2(result0), result1
}

func (adapter intoOperationInstances) FindByID(arg0 context.Context, arg1 int64) (*records.ActivityInstance, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return toNative2(result0), result1
}

func (adapter intoOperationInstances) FindByTenantAndDate(arg0 context.Context, arg1 calendar.Date) ([]*records.ActivityInstance, error) {
	result0, result1 := adapter.value.FindByTenantAndDate(arg0, arg1)
	return toNative3(result0), result1
}
