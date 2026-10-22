//go:build testhooks

package fixturetest

import (
	context "context"

	ports "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

type StudentPickupScheduleRecords interface {
	Create(arg0 context.Context, arg1 *StudentPickupSchedule) error
	DeleteByStudentID(arg0 context.Context, arg1 int64) error
	FindByID(arg0 context.Context, arg1 int64) (*StudentPickupSchedule, error)
	FindByStudentID(arg0 context.Context, arg1 int64) ([]*StudentPickupSchedule, error)
	FindByStudentIDAndWeekday(arg0 context.Context, arg1 int64, arg2 int) (*StudentPickupSchedule, error)
	FindByStudentIDsAndWeekday(arg0 context.Context, arg1 []int64, arg2 int) ([]*StudentPickupSchedule, error)
	Update(arg0 context.Context, arg1 *StudentPickupSchedule) error
	UpsertSchedule(arg0 context.Context, arg1 *StudentPickupSchedule) error
}

type fromStudentPickupScheduleRecords struct {
	value ports.StudentPickupScheduleRecords
}

func FromStudentPickupScheduleRecords(value ports.StudentPickupScheduleRecords) StudentPickupScheduleRecords {
	if value == nil {
		return nil
	}
	return fromStudentPickupScheduleRecords{value: value}
}

func (adapter fromStudentPickupScheduleRecords) Create(arg0 context.Context, arg1 *StudentPickupSchedule) error {
	result0 := adapter.value.Create(arg0, toNative20(arg1))
	return result0
}

func (adapter fromStudentPickupScheduleRecords) DeleteByStudentID(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.DeleteByStudentID(arg0, arg1)
	return result0
}

func (adapter fromStudentPickupScheduleRecords) FindByID(arg0 context.Context, arg1 int64) (*StudentPickupSchedule, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return fromNative20(result0), result1
}

func (adapter fromStudentPickupScheduleRecords) FindByStudentID(arg0 context.Context, arg1 int64) ([]*StudentPickupSchedule, error) {
	result0, result1 := adapter.value.FindByStudentID(arg0, arg1)
	return fromNative21(result0), result1
}

func (adapter fromStudentPickupScheduleRecords) FindByStudentIDAndWeekday(arg0 context.Context, arg1 int64, arg2 int) (*StudentPickupSchedule, error) {
	result0, result1 := adapter.value.FindByStudentIDAndWeekday(arg0, arg1, arg2)
	return fromNative20(result0), result1
}

func (adapter fromStudentPickupScheduleRecords) FindByStudentIDsAndWeekday(arg0 context.Context, arg1 []int64, arg2 int) ([]*StudentPickupSchedule, error) {
	result0, result1 := adapter.value.FindByStudentIDsAndWeekday(arg0, arg1, arg2)
	return fromNative21(result0), result1
}

func (adapter fromStudentPickupScheduleRecords) Update(arg0 context.Context, arg1 *StudentPickupSchedule) error {
	result0 := adapter.value.Update(arg0, toNative20(arg1))
	return result0
}

func (adapter fromStudentPickupScheduleRecords) UpsertSchedule(arg0 context.Context, arg1 *StudentPickupSchedule) error {
	result0 := adapter.value.UpsertSchedule(arg0, toNative20(arg1))
	return result0
}
