package students

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/models/users"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	userService "github.com/moto-nrw/project-phoenix/services/users"
)

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
	students  userService.StudentService
	authorize func(ctx context.Context, student *users.Student, status string) bool
	locked    map[int64]*users.Student
}

func newStatusDayStudents(students userService.StudentService, authorize func(ctx context.Context, student *users.Student, status string) bool) *statusDayStudents {
	return &statusDayStudents{students: students, authorize: authorize, locked: map[int64]*users.Student{}}
}

func (s *statusDayStudents) LockForStatusWrite(ctx context.Context, studentID int64, status string) (*activeService.StudentRecord, error) {
	if s.authorize == nil {
		return nil, errors.New("status day students: authorization callback is required")
	}
	fresh, err := s.students.GetByIDForUpdate(ctx, studentID)
	if err != nil {
		return nil, err
	}
	if !s.authorize(ctx, fresh, status) {
		return nil, activeService.ErrStudentStatusDayReassigned
	}
	s.locked[studentID] = fresh
	return studentRecord(fresh), nil
}

func (s *statusDayStudents) UpdateLiveStatus(ctx context.Context, record *activeService.StudentRecord) error {
	if record == nil {
		return nil
	}
	row := s.locked[record.ID]
	if row == nil {
		fresh, err := s.students.GetByIDForUpdate(ctx, record.ID)
		if err != nil {
			return err
		}
		row = fresh
	}
	row.Sick = record.Sick
	row.SickSince = record.SickSince
	row.Excused = record.Excused
	row.ExcusedSince = record.ExcusedSince
	return s.students.Update(ctx, row)
}

// studentLifecycle maps the owner's lifecycle status onto the states the
// presence flows branch on. Every other status, today only "pending", is
// StudentLifecycleOther and counts as not-yet-active.
func studentLifecycle(status users.StudentStatus) activeService.StudentLifecycle {
	switch status {
	case users.StudentStatusActive:
		return activeService.StudentLifecycleActive
	case users.StudentStatusInactive:
		return activeService.StudentLifecycleInactive
	case users.StudentStatusAlumnus:
		return activeService.StudentLifecycleAlumnus
	case users.StudentStatusPending:
		return activeService.StudentLifecycleOther
	default:
		return activeService.StudentLifecycleOther
	}
}

func studentRecord(row *users.Student) *activeService.StudentRecord {
	if row == nil {
		return nil
	}
	return &activeService.StudentRecord{
		ID:            row.ID,
		TenantID:      row.TenantID,
		PersonID:      row.PersonID,
		GroupID:       row.GroupID,
		SchoolClass:   row.SchoolClass,
		Lifecycle:     studentLifecycle(row.Status),
		EnrolledFrom:  row.EnrolledFrom,
		EnrolledUntil: row.EnrolledUntil,
		Sick:          row.Sick,
		SickSince:     row.SickSince,
		Excused:       row.Excused,
		ExcusedSince:  row.ExcusedSince,
	}
}
