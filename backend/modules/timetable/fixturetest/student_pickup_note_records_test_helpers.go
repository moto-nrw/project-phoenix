//go:build testhooks

package fixturetest

import (
	context "context"

	ports "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type StudentPickupNoteRecords interface {
	Create(arg0 context.Context, arg1 *StudentPickupNote) error
	DeleteByStudentID(arg0 context.Context, arg1 int64) error
	DeletePastNotes(arg0 context.Context, arg1 calendar.Date) (int64, error)
	FindByID(arg0 context.Context, arg1 int64) (*StudentPickupNote, error)
	FindByStudentID(arg0 context.Context, arg1 int64) ([]*StudentPickupNote, error)
	FindByStudentIDAndDate(arg0 context.Context, arg1 int64, arg2 calendar.Date) ([]*StudentPickupNote, error)
	FindByStudentIDsAndDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]*StudentPickupNote, error)
	Update(arg0 context.Context, arg1 *StudentPickupNote) error
}

type fromStudentPickupNoteRecords struct {
	value ports.StudentPickupNoteRecords
}

func FromStudentPickupNoteRecords(value ports.StudentPickupNoteRecords) StudentPickupNoteRecords {
	if original, ok := value.(interface {
		fixtureStudentPickupNoteRecords() StudentPickupNoteRecords
	}); ok {
		return original.fixtureStudentPickupNoteRecords()
	}
	if value == nil {
		return nil
	}
	return fromStudentPickupNoteRecords{value: value}
}

func (adapter fromStudentPickupNoteRecords) Create(arg0 context.Context, arg1 *StudentPickupNote) error {
	result0 := adapter.value.Create(arg0, toNative18(arg1))
	return result0
}

func (adapter fromStudentPickupNoteRecords) DeleteByStudentID(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.DeleteByStudentID(arg0, arg1)
	return result0
}

func (adapter fromStudentPickupNoteRecords) DeletePastNotes(arg0 context.Context, arg1 calendar.Date) (int64, error) {
	result0, result1 := adapter.value.DeletePastNotes(arg0, arg1)
	return result0, result1
}

func (adapter fromStudentPickupNoteRecords) FindByID(arg0 context.Context, arg1 int64) (*StudentPickupNote, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return fromNative18(result0), result1
}

func (adapter fromStudentPickupNoteRecords) FindByStudentID(arg0 context.Context, arg1 int64) ([]*StudentPickupNote, error) {
	result0, result1 := adapter.value.FindByStudentID(arg0, arg1)
	return fromNative19(result0), result1
}

func (adapter fromStudentPickupNoteRecords) FindByStudentIDAndDate(arg0 context.Context, arg1 int64, arg2 calendar.Date) ([]*StudentPickupNote, error) {
	result0, result1 := adapter.value.FindByStudentIDAndDate(arg0, arg1, arg2)
	return fromNative19(result0), result1
}

func (adapter fromStudentPickupNoteRecords) FindByStudentIDsAndDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]*StudentPickupNote, error) {
	result0, result1 := adapter.value.FindByStudentIDsAndDate(arg0, arg1, arg2)
	return fromNative19(result0), result1
}

func (adapter fromStudentPickupNoteRecords) Update(arg0 context.Context, arg1 *StudentPickupNote) error {
	result0 := adapter.value.Update(arg0, toNative18(arg1))
	return result0
}

func (adapter fromStudentPickupNoteRecords) nativeRecordSource() any { return adapter.value }
