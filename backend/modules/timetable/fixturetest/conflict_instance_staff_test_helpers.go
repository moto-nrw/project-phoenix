//go:build testhooks

package fixturetest

import (
	context "context"

	records "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	ports "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

type ConflictInstanceStaff interface {
	FindByInstanceID(arg0 context.Context, arg1 int64) ([]*InstanceStaff, error)
	FindByInstanceIDs(arg0 context.Context, arg1 []int64) ([]*InstanceStaff, error)
}

type intoConflictInstanceStaff struct {
	value ConflictInstanceStaff
}

func IntoConflictInstanceStaff(value ConflictInstanceStaff) ports.ConflictInstanceStaff {
	if original, ok := value.(interface{ nativeRecordSource() any }); ok {
		if capability, ok := original.nativeRecordSource().(ports.ConflictInstanceStaff); ok {
			return capability
		}
	}
	if original, ok := value.(interface {
		nativeConflictInstanceStaff() ports.ConflictInstanceStaff
	}); ok {
		return original.nativeConflictInstanceStaff()
	}
	if value == nil {
		return nil
	}
	return intoConflictInstanceStaff{value: value}
}

func (adapter intoConflictInstanceStaff) FindByInstanceID(arg0 context.Context, arg1 int64) ([]*records.InstanceStaff, error) {
	result0, result1 := adapter.value.FindByInstanceID(arg0, arg1)
	return toNative6(result0), result1
}

func (adapter intoConflictInstanceStaff) FindByInstanceIDs(arg0 context.Context, arg1 []int64) ([]*records.InstanceStaff, error) {
	result0, result1 := adapter.value.FindByInstanceIDs(arg0, arg1)
	return toNative6(result0), result1
}
