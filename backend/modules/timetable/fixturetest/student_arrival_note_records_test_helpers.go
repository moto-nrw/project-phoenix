//go:build testhooks

package fixturetest

import (
	context "context"

	ports "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type StudentArrivalNoteRecords interface {
	Create(arg0 context.Context, arg1 *StudentArrivalNote) error
	DeleteByStudentID(arg0 context.Context, arg1 int64) error
	DeletePastNotes(arg0 context.Context, arg1 calendar.Date) (int64, error)
	FindByID(arg0 context.Context, arg1 int64) (*StudentArrivalNote, error)
	FindByStudentID(arg0 context.Context, arg1 int64) ([]*StudentArrivalNote, error)
	FindByStudentIDAndDate(arg0 context.Context, arg1 int64, arg2 calendar.Date) ([]*StudentArrivalNote, error)
	FindByStudentIDsAndDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]*StudentArrivalNote, error)
	Update(arg0 context.Context, arg1 *StudentArrivalNote) error
}

type fromStudentArrivalNoteRecords struct {
	value ports.StudentArrivalNoteRecords
}

func FromStudentArrivalNoteRecords(value ports.StudentArrivalNoteRecords) StudentArrivalNoteRecords {
	if original, ok := value.(interface {
		fixtureStudentArrivalNoteRecords() StudentArrivalNoteRecords
	}); ok {
		return original.fixtureStudentArrivalNoteRecords()
	}
	if value == nil {
		return nil
	}
	return fromStudentArrivalNoteRecords{value: value}
}

func (adapter fromStudentArrivalNoteRecords) Create(arg0 context.Context, arg1 *StudentArrivalNote) error {
	result0 := adapter.value.Create(arg0, toNative41(arg1))
	return result0
}

func (adapter fromStudentArrivalNoteRecords) DeleteByStudentID(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.DeleteByStudentID(arg0, arg1)
	return result0
}

func (adapter fromStudentArrivalNoteRecords) DeletePastNotes(arg0 context.Context, arg1 calendar.Date) (int64, error) {
	result0, result1 := adapter.value.DeletePastNotes(arg0, arg1)
	return result0, result1
}

func (adapter fromStudentArrivalNoteRecords) FindByID(arg0 context.Context, arg1 int64) (*StudentArrivalNote, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return fromNative41(result0), result1
}

func (adapter fromStudentArrivalNoteRecords) FindByStudentID(arg0 context.Context, arg1 int64) ([]*StudentArrivalNote, error) {
	result0, result1 := adapter.value.FindByStudentID(arg0, arg1)
	return fromNative42(result0), result1
}

func (adapter fromStudentArrivalNoteRecords) FindByStudentIDAndDate(arg0 context.Context, arg1 int64, arg2 calendar.Date) ([]*StudentArrivalNote, error) {
	result0, result1 := adapter.value.FindByStudentIDAndDate(arg0, arg1, arg2)
	return fromNative42(result0), result1
}

func (adapter fromStudentArrivalNoteRecords) FindByStudentIDsAndDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]*StudentArrivalNote, error) {
	result0, result1 := adapter.value.FindByStudentIDsAndDate(arg0, arg1, arg2)
	return fromNative42(result0), result1
}

func (adapter fromStudentArrivalNoteRecords) Update(arg0 context.Context, arg1 *StudentArrivalNote) error {
	result0 := adapter.value.Update(arg0, toNative41(arg1))
	return result0
}

func (adapter fromStudentArrivalNoteRecords) nativeRecordSource() any { return adapter.value }
