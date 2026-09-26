package students

import (
	"context"
	"errors"

	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// statusDayStudentRows is the owner surface the status-day write port needs:
// the locked read and the full-row write of a child.
type statusDayStudentRows interface {
	FindStudentRecordForMutation(context.Context, int64) (peopleModule.StudentRecord, error)
	UpdateStudent(context.Context, peopleModule.StudentWrite) (peopleModule.StudentRecord, error)
}

// statusDayStudents serves the status-day write port for one request: the
// locked read re-authorizes the caller on the fresh owner row, and the
// live-flag write goes back through the owner's full-row update on exactly
// that row. The JWT-permission decision stays at the HTTP boundary.
//
// The services composition root carries a twin of this adapter. Neither can
// host a shared helper: the conversion needs the owner's row type and the
// presence record together, and the policy forbids this package from importing
// that composition root. Keep the two in step, including the fail-closed
// behaviour when no authorization callback is supplied.
type statusDayStudents struct {
	students  statusDayStudentRows
	authorize func(ctx context.Context, student *Student, status string) bool
	locked    map[int64]*Student
}

func newStatusDayStudents(students statusDayStudentRows, authorize func(ctx context.Context, student *Student, status string) bool) *statusDayStudents {
	return &statusDayStudents{students: students, authorize: authorize, locked: map[int64]*Student{}}
}

func (s *statusDayStudents) LockForStatusWrite(ctx context.Context, studentID int64, status string) (*studentpresence.StudentRecord, error) {
	if s.authorize == nil {
		return nil, errors.New("status day students: authorization callback is required")
	}
	fresh, err := s.lock(ctx, studentID)
	if err != nil {
		return nil, err
	}
	if !s.authorize(ctx, fresh, status) {
		return nil, studentpresence.ErrStudentStatusDayReassigned
	}
	s.locked[studentID] = fresh
	return fresh.presenceRecord(), nil
}

func (s *statusDayStudents) UpdateLiveStatus(ctx context.Context, record *studentpresence.StudentRecord) error {
	if record == nil {
		return nil
	}
	row := s.locked[record.ID]
	if row == nil {
		fresh, err := s.lock(ctx, record.ID)
		if err != nil {
			return err
		}
		row = fresh
	}
	row.Sick = record.Sick
	row.SickSince = record.SickSince
	row.Excused = record.Excused
	row.ExcusedSince = record.ExcusedSince
	stored, err := s.students.UpdateStudent(ctx, row.write())
	if err != nil {
		return translateStudentWriteError(err)
	}
	row.applyRecord(stored)
	return nil
}

func (s *statusDayStudents) lock(ctx context.Context, studentID int64) (*Student, error) {
	record, err := s.students.FindStudentRecordForMutation(ctx, studentID)
	if isMissingStudent(err) {
		return nil, missingStudentError{err: err}
	}
	if err != nil {
		return nil, err
	}
	return studentFromRecord(record), nil
}

// missingStudentError carries a child the owner would not lock in the
// repository not-found shape (common.IsNotFound) the status-day routes answer
// with 404.
type missingStudentError struct{ err error }

func (e missingStudentError) Error() string { return e.err.Error() }
func (e missingStudentError) Unwrap() error { return e.err }

// RepositoryNotFound marks the not-found sentinel for common.IsNotFound.
func (missingStudentError) RepositoryNotFound() {}
