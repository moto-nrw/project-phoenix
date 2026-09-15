package services

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/active"
)

// statusDayStudentSource is the slice of the student service the status-day
// write transaction needs: the locked read and the owner's row write.
type statusDayStudentSource interface {
	GetByIDForUpdate(context.Context, int64) (*users.Student, error)
	Update(context.Context, *users.Student) error
}

// StatusDayAuthorize decides on the freshly locked row whether the caller may
// write the given status for the student.
//
// There is no nil default. The callback is the locked-row re-authorization the
// status-day write depends on, so a missing one fails every write closed
// rather than quietly waving them through; see errStatusDayAuthorizeMissing.
// A caller that genuinely has no caller identity to check passes
// AllowAllStatusDayWrites and says so at the call site.
type StatusDayAuthorize func(ctx context.Context, student *active.StudentRecord, status string) bool

// AllowAllStatusDayWrites authorizes every status-day write. It exists so a
// non-HTTP entry point, which carries no JWT to re-check, has to name that
// intent instead of expressing it as an omitted argument.
func AllowAllStatusDayWrites(context.Context, *active.StudentRecord, string) bool { return true }

var errStatusDayAuthorizeMissing = errors.New("status day students: authorization callback is required")

type statusDayStudents struct {
	source    statusDayStudentSource
	authorize StatusDayAuthorize
	locked    map[int64]*users.Student
}

// StatusDayStudents serves active.StatusDayStudents for one write context. The
// rows locked through LockForStatusWrite are kept so UpdateLiveStatus writes
// the flags back through the owner's full-row path on exactly that row. One
// adapter belongs to one request; do not share it.
//
// api/students carries its own copy of this adapter. That is not an oversight:
// the policy forbids api/students from importing this composition root, and
// the conversion needs both the owner's row type and the presence record, so
// neither side can host a shared helper. Keep the two in step.
func StatusDayStudents(source statusDayStudentSource, authorize StatusDayAuthorize) active.StatusDayStudents {
	return &statusDayStudents{source: source, authorize: authorize, locked: map[int64]*users.Student{}}
}

// statusDayStudentRepoSource is the repository spelling of the same two
// operations; the repository names its locked read FindByIDForUpdate.
type statusDayStudentRepoSource interface {
	FindByIDForUpdate(context.Context, int64) (*users.Student, error)
	Update(context.Context, *users.Student) error
}

type statusDayStudentRepo struct{ source statusDayStudentRepoSource }

func (r statusDayStudentRepo) GetByIDForUpdate(ctx context.Context, id int64) (*users.Student, error) {
	return r.source.FindByIDForUpdate(ctx, id)
}

func (r statusDayStudentRepo) Update(ctx context.Context, student *users.Student) error {
	return r.source.Update(ctx, student)
}

// StatusDayStudentsFromRepository is StatusDayStudents backed by the student
// repository rather than the student service.
func StatusDayStudentsFromRepository(source statusDayStudentRepoSource, authorize StatusDayAuthorize) active.StatusDayStudents {
	return StatusDayStudents(statusDayStudentRepo{source: source}, authorize)
}

func (s *statusDayStudents) LockForStatusWrite(ctx context.Context, studentID int64, status string) (*active.StudentRecord, error) {
	if s.authorize == nil {
		return nil, errStatusDayAuthorizeMissing
	}
	fresh, err := s.source.GetByIDForUpdate(ctx, studentID)
	if err != nil {
		return nil, err
	}
	record := studentRecord(fresh)
	if !s.authorize(ctx, record, status) {
		return nil, active.ErrStudentStatusDayReassigned
	}
	s.locked[studentID] = fresh
	return record, nil
}

func (s *statusDayStudents) UpdateLiveStatus(ctx context.Context, record *active.StudentRecord) error {
	if record == nil {
		return nil
	}
	row := s.locked[record.ID]
	if row == nil {
		fresh, err := s.source.GetByIDForUpdate(ctx, record.ID)
		if err != nil {
			return err
		}
		row = fresh
	}
	applyStudentLiveStatus(row, record)
	return s.source.Update(ctx, row)
}

// applyStudentLiveStatus copies the presence flags onto the owner's row so a
// caller holding the full row can persist them through the owner's write path.
func applyStudentLiveStatus(row *users.Student, record *active.StudentRecord) {
	if row == nil || record == nil {
		return
	}
	row.Sick = record.Sick
	row.SickSince = record.SickSince
	row.Excused = record.Excused
	row.ExcusedSince = record.ExcusedSince
}
