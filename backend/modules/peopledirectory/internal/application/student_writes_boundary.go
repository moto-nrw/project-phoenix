package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	owner1 "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// StudentEnrollmentStatus is the School Membership half of the child's
// lifecycle the write seam reaches: the status of the enrollment. The
// composition root binds the owner's public capability.
type StudentEnrollmentStatus interface {
	SetStatus(ctx context.Context, studentID int64, status string) (bool, error)
	TransitionStatus(ctx context.Context, studentID int64, expected, next string) (bool, error)
}

// Create inserts the child and reflects the stored row — the assigned id, the
// timestamps and the plan as persisted — back onto the caller's struct, which
// is the contract the retained callers rely on.
func (w *StudentRecords) Create(ctx context.Context, student *peopleModule.StudentState) error {
	if student == nil {
		return errors.New("student cannot be nil")
	}
	if student.PersonID <= 0 {
		return RetainedStudentWriteError(&peopleModule.InvalidStudentError{Reason: "person ID is required"})
	}
	record, err := w.directory.CreateStudent(ctx, toApplicationStudentWrite(StudentWriteFromModel(student)))
	if err != nil {
		return RetainedStudentWriteError(MapError(err))
	}
	ApplyStudentRecord(student, peopleModule.StudentRecord(record))
	return nil
}

func (w *StudentRecords) Update(ctx context.Context, student *peopleModule.StudentState) error {
	if student == nil {
		return errors.New("student cannot be nil")
	}
	if student.ID <= 0 {
		return RetainedStudentWriteError(&peopleModule.InvalidStudentError{Reason: "student ID is required"})
	}
	record, err := w.directory.UpdateStudent(ctx, toApplicationStudentWrite(StudentWriteFromModel(student)))
	if err != nil {
		return RetainedStudentWriteError(MapError(err))
	}
	ApplyStudentRecord(student, peopleModule.StudentRecord(record))
	return nil
}

// SetEnrollmentStatus (the retained UpdateStatus), TransitionStatus,
// SetEnrolledUntilByIDs and SetEnrollmentWindowByID are the child's lifecycle
// and care window. They take the same class-writes gate every other student
// write does, because either can move a child into or out of a class a grade
// transition is mid-way through.
func (w *StudentRecords) UpdateStatus(ctx context.Context, studentID int64, status string) error {
	changed, err := w.membership.SetStatus(ctx, studentID, string(status))
	if err == nil && !changed {
		err = peopleModule.ErrStudentNotFound
	}
	return RetainedStudentWriteError(MapError(err))
}

func (w *StudentRecords) TransitionStatus(
	ctx context.Context,
	studentID int64,
	expected, next string,
) (bool, error) {
	moved, err := w.membership.TransitionStatus(ctx, studentID, string(expected), string(next))
	return moved, RetainedStudentWriteError(MapError(err))
}

func (w *StudentRecords) FindCareBoundsByIDs(ctx context.Context, ids []int64) (map[int64]owner1.Date, error) {
	ids = peopleModule.PositiveIDs(ids)
	if len(ids) == 0 {
		return map[int64]owner1.Date{}, nil
	}
	values, err := w.directory.ListStudentCareEnds(ctx, ids)
	if err != nil {
		return nil, RetainedStudentWriteError(MapError(err))
	}
	bounds := make(map[int64]owner1.Date, len(values))
	for id, value := range values {
		if parsed := peopleModule.OptionalCalendarDate(value); parsed != nil {
			bounds[id] = *parsed
		}
	}
	return bounds, nil
}

func (w *StudentRecords) VerifyCompanionStrandingBatch(ctx context.Context) error {
	return RetainedStudentWriteError(MapError(w.directory.VerifyStrandingBatch(ctx)))
}

func (r *StudentRecords) FindByTeacherIDWithGroups(
	ctx context.Context,
	teacherID int64,
) ([]*peopleModule.StudentRosterEntry, error) {
	if r.teacherGroupIDs == nil {
		return nil, errors.New("student repository resolves teacher assignments through School Membership")
	}
	groupIDs, err := r.teacherGroupIDs(ctx, teacherID)
	if err != nil {
		return nil, err
	}
	groupIDs = peopleModule.PositiveIDs(groupIDs)
	if len(groupIDs) == 0 {
		return []*peopleModule.StudentRosterEntry{}, nil
	}
	return r.roster(ctx, "find by teacher id with groups", func() ([]domain.StudentRosterEntry, error) {
		return r.directory.ListRosterByGroups(ctx, groupIDs, owner1.TodayDate().String())
	})
}

func (r *StudentRecords) FindByTeacherStaffIDsWithGroups(
	ctx context.Context,
	staffIDs []int64,
) ([]*peopleModule.StudentRosterEntry, error) {
	if len(staffIDs) == 0 {
		return []*peopleModule.StudentRosterEntry{}, nil
	}
	if r.teacherStaffGroupIDs == nil {
		return nil, errors.New("student repository resolves teacher assignments through School Membership")
	}
	groupIDs, err := r.teacherStaffGroupIDs(ctx, staffIDs)
	if err != nil {
		return nil, err
	}
	groupIDs = peopleModule.PositiveIDs(groupIDs)
	if len(groupIDs) == 0 {
		return []*peopleModule.StudentRosterEntry{}, nil
	}
	return r.roster(ctx, "find by teacher id with groups", func() ([]domain.StudentRosterEntry, error) {
		return r.directory.ListRosterByGroups(ctx, groupIDs, owner1.TodayDate().String())
	})
}

// FindOverlappingWithGroupsOnDate is the meal-plan boundary: now is an instant
// so the immediate-activation rule uses the actual current Berlin day rather
// than applying the requested list date retroactively.
func (r *StudentRecords) FindOverlappingWithGroupsOnDate(
	ctx context.Context,
	value string,
	now time.Time,
) ([]*peopleModule.StudentRosterEntry, error) {
	date := peopleModule.OptionalCalendarDate(value)
	if date == nil {
		return nil, fmt.Errorf("find students overlapping date: %q is not a calendar date", value)
	}
	return r.FindOverlappingWithGroups(ctx, *date, *date, owner1.DateFromTime(now))
}

func (w *StudentRecords) Delete(ctx context.Context, id any) error {
	studentID, ok := studentIDOf(id)
	if !ok {
		return fmt.Errorf("delete student: unsupported id type %T", id)
	}
	if studentID <= 0 {
		return RetainedStudentWriteError(&peopleModule.InvalidStudentError{Reason: "student ID is required"})
	}
	return RetainedStudentWriteError(MapError(w.directory.DeleteStudent(ctx, studentID)))
}

func studentIDOf(id any) (int64, bool) {
	switch value := id.(type) {
	case int64:
		return value, true
	case int:
		return int64(value), true
	case int32:
		return int64(value), true
	case *int64:
		if value == nil {
			return 0, false
		}
		return *value, true
	default:
		return 0, false
	}
}

func NewStudentRecords(repository peoplePorts.StudentRepository, students *StudentService, membership StudentEnrollmentStatus, teacherGroups func(context.Context, int64) ([]int64, error), teacherStaffGroups func(context.Context, []int64) ([]int64, error)) peoplePorts.StudentRepository {
	return &StudentRecords{StudentRepository: repository, directory: students, membership: membership, teacherGroupIDs: teacherGroups, teacherStaffGroupIDs: teacherStaffGroups}
}
