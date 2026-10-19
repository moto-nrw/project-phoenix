//go:build testhooks

package fixturetest

import (
	context "context"

	ports "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

type StudentArrivalScheduleRecords interface {
	Create(arg0 context.Context, arg1 *StudentArrivalSchedule) error
	DeleteByStudentID(arg0 context.Context, arg1 int64) error
	FindByID(arg0 context.Context, arg1 int64) (*StudentArrivalSchedule, error)
	FindByStudentID(arg0 context.Context, arg1 int64) ([]*StudentArrivalSchedule, error)
	FindByStudentIDAndWeekday(arg0 context.Context, arg1 int64, arg2 int) (*StudentArrivalSchedule, error)
	FindByStudentIDsAndWeekday(arg0 context.Context, arg1 []int64, arg2 int) ([]*StudentArrivalSchedule, error)
	Update(arg0 context.Context, arg1 *StudentArrivalSchedule) error
	UpsertSchedule(arg0 context.Context, arg1 *StudentArrivalSchedule) error
}

type fromStudentArrivalScheduleRecords struct {
	value ports.StudentArrivalScheduleRecords
}

func FromStudentArrivalScheduleRecords(value ports.StudentArrivalScheduleRecords) StudentArrivalScheduleRecords {
	if original, ok := value.(interface {
		fixtureStudentArrivalScheduleRecords() StudentArrivalScheduleRecords
	}); ok {
		return original.fixtureStudentArrivalScheduleRecords()
	}
	if value == nil {
		return nil
	}
	return fromStudentArrivalScheduleRecords{value: value}
}

func (adapter fromStudentArrivalScheduleRecords) Create(arg0 context.Context, arg1 *StudentArrivalSchedule) error {
	result0 := adapter.value.Create(arg0, toNative14(arg1))
	return result0
}

func (adapter fromStudentArrivalScheduleRecords) DeleteByStudentID(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.DeleteByStudentID(arg0, arg1)
	return result0
}

func (adapter fromStudentArrivalScheduleRecords) FindByID(arg0 context.Context, arg1 int64) (*StudentArrivalSchedule, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return fromNative14(result0), result1
}

func (adapter fromStudentArrivalScheduleRecords) FindByStudentID(arg0 context.Context, arg1 int64) ([]*StudentArrivalSchedule, error) {
	result0, result1 := adapter.value.FindByStudentID(arg0, arg1)
	return fromNative15(result0), result1
}

func (adapter fromStudentArrivalScheduleRecords) FindByStudentIDAndWeekday(arg0 context.Context, arg1 int64, arg2 int) (*StudentArrivalSchedule, error) {
	result0, result1 := adapter.value.FindByStudentIDAndWeekday(arg0, arg1, arg2)
	return fromNative14(result0), result1
}

func (adapter fromStudentArrivalScheduleRecords) FindByStudentIDsAndWeekday(arg0 context.Context, arg1 []int64, arg2 int) ([]*StudentArrivalSchedule, error) {
	result0, result1 := adapter.value.FindByStudentIDsAndWeekday(arg0, arg1, arg2)
	return fromNative15(result0), result1
}

func (adapter fromStudentArrivalScheduleRecords) Update(arg0 context.Context, arg1 *StudentArrivalSchedule) error {
	result0 := adapter.value.Update(arg0, toNative14(arg1))
	return result0
}

func (adapter fromStudentArrivalScheduleRecords) UpsertSchedule(arg0 context.Context, arg1 *StudentArrivalSchedule) error {
	result0 := adapter.value.UpsertSchedule(arg0, toNative14(arg1))
	return result0
}

func (adapter fromStudentArrivalScheduleRecords) nativeRecordSource() any { return adapter.value }
