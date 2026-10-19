//go:build testhooks

package fixturetest

import (
	context "context"

	records "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	ports "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

type OperationInstanceStaff interface {
	FindByInstanceID(arg0 context.Context, arg1 int64) ([]*InstanceStaff, error)
	FindByInstanceIDs(arg0 context.Context, arg1 []int64) ([]*InstanceStaff, error)
}

type intoOperationInstanceStaff struct {
	value OperationInstanceStaff
}

func IntoOperationInstanceStaff(value OperationInstanceStaff) ports.OperationInstanceStaff {
	if original, ok := value.(interface{ nativeRecordSource() any }); ok {
		if capability, ok := original.nativeRecordSource().(ports.OperationInstanceStaff); ok {
			return capability
		}
	}
	if original, ok := value.(interface {
		nativeOperationInstanceStaff() ports.OperationInstanceStaff
	}); ok {
		return original.nativeOperationInstanceStaff()
	}
	if value == nil {
		return nil
	}
	return intoOperationInstanceStaff{value: value}
}

func (adapter intoOperationInstanceStaff) FindByInstanceID(arg0 context.Context, arg1 int64) ([]*records.InstanceStaff, error) {
	result0, result1 := adapter.value.FindByInstanceID(arg0, arg1)
	return toNative6(result0), result1
}

func (adapter intoOperationInstanceStaff) FindByInstanceIDs(arg0 context.Context, arg1 []int64) ([]*records.InstanceStaff, error) {
	result0, result1 := adapter.value.FindByInstanceIDs(arg0, arg1)
	return toNative6(result0), result1
}
