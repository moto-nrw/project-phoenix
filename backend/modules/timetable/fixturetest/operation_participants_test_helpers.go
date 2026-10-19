//go:build testhooks

package fixturetest

import (
	context "context"

	records "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	ports "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type OperationParticipants interface {
	FindByInstanceAndStudent(arg0 context.Context, arg1 int64, arg2 int64) (*InstanceStudent, error)
	FindByInstanceID(arg0 context.Context, arg1 int64) ([]*InstanceStudent, error)
	FindByInstanceIDs(arg0 context.Context, arg1 []int64) ([]*InstanceStudent, error)
	FindPresentInOtherActiveInstances(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 []int64) ([]ParallelPresence, error)
	UpdateAttendanceFields(arg0 context.Context, arg1 int64, arg2 AttendanceFieldPatch) error
}

type intoOperationParticipants struct {
	value OperationParticipants
}

func IntoOperationParticipants(value OperationParticipants) ports.OperationParticipants {
	if original, ok := value.(interface{ nativeRecordSource() any }); ok {
		if capability, ok := original.nativeRecordSource().(ports.OperationParticipants); ok {
			return capability
		}
	}
	if original, ok := value.(interface {
		nativeOperationParticipants() ports.OperationParticipants
	}); ok {
		return original.nativeOperationParticipants()
	}
	if value == nil {
		return nil
	}
	return intoOperationParticipants{value: value}
}

func (adapter intoOperationParticipants) FindByInstanceAndStudent(arg0 context.Context, arg1 int64, arg2 int64) (*records.InstanceStudent, error) {
	result0, result1 := adapter.value.FindByInstanceAndStudent(arg0, arg1, arg2)
	return toNative27(result0), result1
}

func (adapter intoOperationParticipants) FindByInstanceID(arg0 context.Context, arg1 int64) ([]*records.InstanceStudent, error) {
	result0, result1 := adapter.value.FindByInstanceID(arg0, arg1)
	return toNative26(result0), result1
}

func (adapter intoOperationParticipants) FindByInstanceIDs(arg0 context.Context, arg1 []int64) ([]*records.InstanceStudent, error) {
	result0, result1 := adapter.value.FindByInstanceIDs(arg0, arg1)
	return toNative26(result0), result1
}

func (adapter intoOperationParticipants) FindPresentInOtherActiveInstances(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 []int64) ([]records.ParallelPresence, error) {
	result0, result1 := adapter.value.FindPresentInOtherActiveInstances(arg0, arg1, arg2, arg3)
	return toNative32(result0), result1
}

func (adapter intoOperationParticipants) UpdateAttendanceFields(arg0 context.Context, arg1 int64, arg2 records.AttendanceFieldPatch) error {
	result0 := adapter.value.UpdateAttendanceFields(arg0, arg1, fromNative28(arg2))
	return result0
}
