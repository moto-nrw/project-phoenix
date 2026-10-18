package api

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// StudentChangeAudit appends the tracked-field history of a child row,
// attributed to the acting account.
type StudentChangeAudit interface {
	RecordChangesForActor(ctx context.Context, before, after *peopledirectory.StudentState, editedBy int64) error
}

// companionStudentRows are the People Directory child rows the companion
// graph locks, reads and updates.
type companionStudentRows interface {
	FindByID(ctx context.Context, id any) (*peopledirectory.StudentState, error)
	FindByIDForUpdate(ctx context.Context, id int64) (*peopledirectory.StudentState, error)
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*peopledirectory.StudentState, error)
	Update(ctx context.Context, student *peopledirectory.StudentState) error
}

// CompanionStudents serves the People Directory half of the Care Plan
// companion graph (#3427): the child rows every edge writer locks, the
// departure plans the companion rules read, and the one write into another
// child's plan that a confirmed extension makes.
type CompanionStudents struct {
	students companionStudentRows
	people   CompanionStudentLock
	writes   peopleCompose.AuditedStudentUpdates
}

// CompanionStudentLock is the People Directory owner's refusing row lock: a
// held lock fails at once instead of waiting.
type CompanionStudentLock interface {
	FindStudentRecordForMutationNoWait(context.Context, int64) (peopledirectory.StudentRecord, error)
}

// NewCompanionStudents binds the port. The audit may be nil only in tests
// that never widen a companion's plan: the write refuses an unattributed
// widening rather than skipping the trail.
func NewCompanionStudents(students companionStudentRows, people CompanionStudentLock, audit StudentChangeAudit) carePlanCompose.CompanionStudents {
	return CompanionStudents{students: students, people: people, writes: peopleCompose.NewAuditedStudentUpdates(students, audit)}
}

func companionStudent(row *peopledirectory.StudentState) carePlanCompose.CompanionStudent {
	return carePlanCompose.CompanionStudent{
		ID:               row.ID,
		HasCompanionNote: peopledirectory.HasCompanionNote(row.DepartureCompanionNote),
		AccompaniedDays:  peopledirectory.AccompaniedWeekdays(row.AllowedDepartureModes, row.DepartureDays),
	}
}

func (s CompanionStudents) FindCompanionStudent(ctx context.Context, studentID int64) (*carePlanCompose.CompanionStudent, error) {
	row, err := s.students.FindByID(ctx, studentID)
	if err != nil || row == nil {
		return nil, err
	}
	student := companionStudent(row)
	return &student, nil
}

func (s CompanionStudents) FindCompanionStudents(ctx context.Context, studentIDs []int64) (map[int64]carePlanCompose.CompanionStudent, error) {
	rows, err := s.students.FindByIDs(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]carePlanCompose.CompanionStudent, len(rows))
	for id, row := range rows {
		if row != nil {
			result[id] = companionStudent(row)
		}
	}
	return result, nil
}

// LockCompanionStudent takes the row lock. A missing row is skipped: the lock
// pass establishes order and nothing else, and validation tells a missing
// subject from a missing companion.
func (s CompanionStudents) LockCompanionStudent(ctx context.Context, studentID int64, noWait bool) error {
	var err error
	if noWait {
		_, err = s.people.FindStudentRecordForMutationNoWait(ctx, studentID)
	} else {
		_, err = s.students.FindByIDForUpdate(ctx, studentID)
	}
	switch {
	case err == nil, peoplePorts.IsRepositoryNoRows(err), errors.Is(err, peopledirectory.ErrStudentNotFound):
		return nil
	case errors.Is(err, peopledirectory.ErrStudentLockBusy):
		return careplan.ErrCompanionLockBusy
	default:
		return err
	}
}

// ExtendAccompaniedDays widens one child's allowed departure modes so the
// days permit leaving with another child (People Directory's locked
// WidenAccompaniedDaysByID). Purely additive: the bus and pickup permissions
// of those days stay untouched. A missing child is Care Plan's
// ErrCompanionNotFound.
func (s CompanionStudents) ExtendAccompaniedDays(ctx context.Context, studentID int64, days []string, actorAccountID int64) error {
	found, err := s.writes.WidenAccompaniedDaysByID(ctx, studentID, days, actorAccountID)
	if err == nil && !found {
		return careplan.ErrCompanionNotFound
	}
	return err
}
