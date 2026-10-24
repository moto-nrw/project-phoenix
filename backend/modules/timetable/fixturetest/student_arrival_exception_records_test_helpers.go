//go:build testhooks

package fixturetest

import (
	context "context"

	ports "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type StudentArrivalExceptionRecords interface {
	Create(arg0 context.Context, arg1 *StudentArrivalException) error
	DeleteByStudentID(arg0 context.Context, arg1 int64) error
	FindByID(arg0 context.Context, arg1 int64) (*StudentArrivalException, error)
	FindByStudentID(arg0 context.Context, arg1 int64) ([]*StudentArrivalException, error)
	FindByStudentIDAndDate(arg0 context.Context, arg1 int64, arg2 calendar.Date) (*StudentArrivalException, error)
	FindByStudentIDsAndDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]*StudentArrivalException, error)
	FindUpcomingByStudentID(arg0 context.Context, arg1 int64) ([]*StudentArrivalException, error)
	Update(arg0 context.Context, arg1 *StudentArrivalException) error
}

type fromStudentArrivalExceptionRecords struct {
	value ports.StudentArrivalExceptionRecords
}

func FromStudentArrivalExceptionRecords(value ports.StudentArrivalExceptionRecords) StudentArrivalExceptionRecords {
	if value == nil {
		return nil
	}
	return fromStudentArrivalExceptionRecords{value: value}
}

func (adapter fromStudentArrivalExceptionRecords) Create(arg0 context.Context, arg1 *StudentArrivalException) error {
	result0 := adapter.value.Create(arg0, toNative12(arg1))
	return result0
}

func (adapter fromStudentArrivalExceptionRecords) DeleteByStudentID(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.DeleteByStudentID(arg0, arg1)
	return result0
}

func (adapter fromStudentArrivalExceptionRecords) FindByID(arg0 context.Context, arg1 int64) (*StudentArrivalException, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return fromNative12(result0), result1
}

func (adapter fromStudentArrivalExceptionRecords) FindByStudentID(arg0 context.Context, arg1 int64) ([]*StudentArrivalException, error) {
	result0, result1 := adapter.value.FindByStudentID(arg0, arg1)
	return fromNative13(result0), result1
}

func (adapter fromStudentArrivalExceptionRecords) FindByStudentIDAndDate(arg0 context.Context, arg1 int64, arg2 calendar.Date) (*StudentArrivalException, error) {
	result0, result1 := adapter.value.FindByStudentIDAndDate(arg0, arg1, arg2)
	return fromNative12(result0), result1
}

func (adapter fromStudentArrivalExceptionRecords) FindByStudentIDsAndDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]*StudentArrivalException, error) {
	result0, result1 := adapter.value.FindByStudentIDsAndDate(arg0, arg1, arg2)
	return fromNative13(result0), result1
}

func (adapter fromStudentArrivalExceptionRecords) FindUpcomingByStudentID(arg0 context.Context, arg1 int64) ([]*StudentArrivalException, error) {
	result0, result1 := adapter.value.FindUpcomingByStudentID(arg0, arg1)
	return fromNative13(result0), result1
}

func (adapter fromStudentArrivalExceptionRecords) Update(arg0 context.Context, arg1 *StudentArrivalException) error {
	result0 := adapter.value.Update(arg0, toNative12(arg1))
	return result0
}

func (adapter fromStudentArrivalExceptionRecords) nativeRecordSource() any { return adapter.value }
