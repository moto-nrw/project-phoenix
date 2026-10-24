//go:build testhooks

package fixturetest

import (
	context "context"

	ports "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type StudentPickupExceptionRecords interface {
	Create(arg0 context.Context, arg1 *StudentPickupException) error
	DeleteByStudentID(arg0 context.Context, arg1 int64) error
	FindByID(arg0 context.Context, arg1 int64) (*StudentPickupException, error)
	FindByStudentID(arg0 context.Context, arg1 int64) ([]*StudentPickupException, error)
	FindByStudentIDAndDate(arg0 context.Context, arg1 int64, arg2 calendar.Date) (*StudentPickupException, error)
	FindByStudentIDsAndDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]*StudentPickupException, error)
	FindUpcomingByStudentID(arg0 context.Context, arg1 int64) ([]*StudentPickupException, error)
	Update(arg0 context.Context, arg1 *StudentPickupException) error
}

type fromStudentPickupExceptionRecords struct {
	value ports.StudentPickupExceptionRecords
}

func FromStudentPickupExceptionRecords(value ports.StudentPickupExceptionRecords) StudentPickupExceptionRecords {
	if value == nil {
		return nil
	}
	return fromStudentPickupExceptionRecords{value: value}
}

func (adapter fromStudentPickupExceptionRecords) Create(arg0 context.Context, arg1 *StudentPickupException) error {
	result0 := adapter.value.Create(arg0, toNative16(arg1))
	return result0
}

func (adapter fromStudentPickupExceptionRecords) DeleteByStudentID(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.DeleteByStudentID(arg0, arg1)
	return result0
}

func (adapter fromStudentPickupExceptionRecords) FindByID(arg0 context.Context, arg1 int64) (*StudentPickupException, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return fromNative16(result0), result1
}

func (adapter fromStudentPickupExceptionRecords) FindByStudentID(arg0 context.Context, arg1 int64) ([]*StudentPickupException, error) {
	result0, result1 := adapter.value.FindByStudentID(arg0, arg1)
	return fromNative17(result0), result1
}

func (adapter fromStudentPickupExceptionRecords) FindByStudentIDAndDate(arg0 context.Context, arg1 int64, arg2 calendar.Date) (*StudentPickupException, error) {
	result0, result1 := adapter.value.FindByStudentIDAndDate(arg0, arg1, arg2)
	return fromNative16(result0), result1
}

func (adapter fromStudentPickupExceptionRecords) FindByStudentIDsAndDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]*StudentPickupException, error) {
	result0, result1 := adapter.value.FindByStudentIDsAndDate(arg0, arg1, arg2)
	return fromNative17(result0), result1
}

func (adapter fromStudentPickupExceptionRecords) FindUpcomingByStudentID(arg0 context.Context, arg1 int64) ([]*StudentPickupException, error) {
	result0, result1 := adapter.value.FindUpcomingByStudentID(arg0, arg1)
	return fromNative17(result0), result1
}

func (adapter fromStudentPickupExceptionRecords) Update(arg0 context.Context, arg1 *StudentPickupException) error {
	result0 := adapter.value.Update(arg0, toNative16(arg1))
	return result0
}
