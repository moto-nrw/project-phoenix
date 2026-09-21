package repositories

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// StudentChangeAudit appends the tracked-field history of a child row,
// attributed to the acting account.
type StudentChangeAudit interface {
	RecordChangesForActor(ctx context.Context, before, after *users.Student, editedBy int64) error
}

// CompanionStudents serves the People Directory half of the Care Plan
// companion graph (#3427): the child rows every edge writer locks, the
// departure plans the companion rules read, and the one write into another
// child's plan that a confirmed extension makes.
type CompanionStudents struct {
	students users.StudentRepository
	people   CompanionStudentLock
	audit    StudentChangeAudit
}

// CompanionStudentLock is the People Directory owner's refusing row lock: a
// held lock fails at once instead of waiting.
type CompanionStudentLock interface {
	FindStudentRecordForMutationNoWait(context.Context, int64) (peopledirectory.StudentRecord, error)
}

// NewCompanionStudents binds the port. The audit may be nil only in tests
// that never widen a companion's plan: the write refuses an unattributed
// widening rather than skipping the trail.
func NewCompanionStudents(students users.StudentRepository, people CompanionStudentLock, audit StudentChangeAudit) carePlanCompose.CompanionStudents {
	return CompanionStudents{students: students, people: people, audit: audit}
}

func companionStudent(row *users.Student) carePlanCompose.CompanionStudent {
	return carePlanCompose.CompanionStudent{
		ID:               row.ID,
		HasCompanionNote: row.DepartureCompanionNote != nil && strings.TrimSpace(*row.DepartureCompanionNote) != "",
		AccompaniedDays:  users.AccompaniedWeekdays(row.AllowedDepartureModes, row.DepartureDays),
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
	case err == nil, usersRepo.IsNotFound(err), errors.Is(err, peopledirectory.ErrStudentNotFound):
		return nil
	case errors.Is(err, peopledirectory.ErrStudentLockBusy):
		return careplan.ErrCompanionLockBusy
	default:
		return err
	}
}

// ExtendAccompaniedDays widens one child's allowed departure modes so the
// days permit leaving with another child. Purely additive: the bus and
// pickup permissions of those days stay untouched.
//
// The row is re-read under its lock rather than reused from an unlocked
// snapshot: widening writes the WHOLE student row, so a change another request
// committed in between would otherwise be rolled back by a stale write.
func (s CompanionStudents) ExtendAccompaniedDays(ctx context.Context, studentID int64, days []string, actorAccountID int64) error {
	companion, err := s.students.FindByIDForUpdate(ctx, studentID)
	if err != nil {
		if usersRepo.IsNotFound(err) {
			return careplan.ErrCompanionNotFound
		}
		return err
	}
	if companion == nil {
		return careplan.ErrCompanionNotFound
	}
	allowed := users.AccompaniedWeekdays(companion.AllowedDepartureModes, companion.DepartureDays)
	var missing []string
	for _, day := range careplan.CompanionWeekdays {
		if slices.Contains(days, day) && !allowed[day] {
			missing = append(missing, day)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	if s.audit != nil && actorAccountID <= 0 {
		return errors.New("users: companion departure audit requires an actor account")
	}
	before := *companion
	companion.AllowedDepartureModes = users.WithAccompaniedDays(companion.AllowedDepartureModes, missing)
	// The links about to be written ARE the "mit wem" detail for exactly these
	// weekdays. Days the companion already had accompanied stay uncovered on
	// purpose: the repository probes the stored edges for those.
	companion.MarkDepartureCompanionDays(days...)
	if err := s.students.Update(ctx, companion); err != nil {
		return err
	}
	if s.audit != nil {
		if err := s.audit.RecordChangesForActor(ctx, &before, companion, actorAccountID); err != nil {
			return fmt.Errorf("users: audit companion departure plan: %w", err)
		}
	}
	return nil
}
