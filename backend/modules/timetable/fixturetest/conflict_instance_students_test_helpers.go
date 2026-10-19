//go:build testhooks

package fixturetest

import (
	context "context"

	records "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	ports "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

type ConflictInstanceStudents interface {
	FindByInstanceID(arg0 context.Context, arg1 int64) ([]*InstanceStudent, error)
	FindExpectedByInstanceIDs(arg0 context.Context, arg1 []int64) ([]*InstanceStudent, error)
}

type intoConflictInstanceStudents struct {
	value ConflictInstanceStudents
}

func IntoConflictInstanceStudents(value ConflictInstanceStudents) ports.ConflictInstanceStudents {
	if original, ok := value.(interface{ nativeRecordSource() any }); ok {
		if capability, ok := original.nativeRecordSource().(ports.ConflictInstanceStudents); ok {
			return capability
		}
	}
	if original, ok := value.(interface {
		nativeConflictInstanceStudents() ports.ConflictInstanceStudents
	}); ok {
		return original.nativeConflictInstanceStudents()
	}
	if value == nil {
		return nil
	}
	return intoConflictInstanceStudents{value: value}
}

func (adapter intoConflictInstanceStudents) FindByInstanceID(arg0 context.Context, arg1 int64) ([]*records.InstanceStudent, error) {
	result0, result1 := adapter.value.FindByInstanceID(arg0, arg1)
	return toNative26(result0), result1
}

func (adapter intoConflictInstanceStudents) FindExpectedByInstanceIDs(arg0 context.Context, arg1 []int64) ([]*records.InstanceStudent, error) {
	result0, result1 := adapter.value.FindExpectedByInstanceIDs(arg0, arg1)
	return toNative26(result0), result1
}
