package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	users "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// StudentRowUpdate is the retained child row write an audited update runs.
type StudentRowUpdate interface {
	Update(ctx context.Context, student *users.Student) error
}

// StudentRowLockUpdate is the retained child row a by-ID write re-reads under
// its lock and then writes.
type StudentRowLockUpdate interface {
	StudentRowUpdate
	FindByIDForUpdate(ctx context.Context, id int64) (*users.Student, error)
}

// StudentChangeActorRecorder appends the tracked-field history of one child
// write, attributed to the acting account.
type StudentChangeActorRecorder interface {
	RecordChangesForActor(ctx context.Context, before, after *users.Student, editedBy int64) error
}

// AuditedStudentUpdates writes a child row and then appends the change history
// of that write, in this order and inside the caller's transaction, so the
// history commits or rolls back with the row it describes. Without a recorder
// the history is skipped, as graphs without one always did.
type AuditedStudentUpdates struct {
	students StudentRowUpdate
	locks    StudentRowLockUpdate
	audit    StudentChangeActorRecorder
}

// NewLockedStudentUpdates binds the command together with the row lock its
// by-ID writes take. audit may be nil.
func NewLockedStudentUpdates(students StudentRowLockUpdate, audit StudentChangeActorRecorder) AuditedStudentUpdates {
	return AuditedStudentUpdates{students: students, locks: students, audit: audit}
}

// UpdateStudentAudited stores after and records before against it for
// actorAccountID. The write and the history share one companion recording
// scope, so the result reports whether the write trimmed a "läuft mit" link.
// A failed history comes back as a *peopledirectory.StudentAuditError.
func (u AuditedStudentUpdates) UpdateStudentAudited(ctx context.Context, before, after *users.Student, actorAccountID int64) (bool, error) {
	writeCtx, companions := users.ContextWithCompanionChangeRecorder(ctx)
	if err := u.write(writeCtx, before, after, actorAccountID); err != nil {
		return false, err
	}
	return companions.Changed(), nil
}

// WidenAccompaniedDays lets the locked companion row leave with another child
// on days: it adds the days its plan does not yet allow and marks the links
// about to be written as the "mit wem" detail of exactly those weekdays. Days
// the companion already had accompanied stay unmarked on purpose: the
// repository probes the stored edges for those. A plan that already allows
// every day is not written. With a recorder bound, an unattributed widening is
// refused rather than written without its history.
func (u AuditedStudentUpdates) WidenAccompaniedDays(ctx context.Context, companion *users.Student, days []string, actorAccountID int64) error {
	missing := peopledirectory.MissingAccompaniedDays(companion.AllowedDepartureModes, companion.DepartureDays, days)
	if len(missing) == 0 {
		return nil
	}
	if u.audit != nil && actorAccountID <= 0 {
		return errors.New("users: companion departure audit requires an actor account")
	}
	before := *companion
	companion.AllowedDepartureModes = users.WithAccompaniedDays(companion.AllowedDepartureModes, missing)
	companion.MarkDepartureCompanionDays(days...)
	err := u.write(ctx, &before, companion, actorAccountID)
	var auditErr *peopledirectory.StudentAuditError
	if errors.As(err, &auditErr) {
		return fmt.Errorf("users: audit companion departure plan: %w", auditErr.Err)
	}
	return err
}

// WidenAccompaniedDaysByID re-reads the companion under its lock and widens
// it (WidenAccompaniedDays). The row is re-read rather than reused from an
// unlocked snapshot: widening writes the WHOLE student row, so a change another
// request committed in between would otherwise be rolled back by a stale
// write. found is false when the child is not there; nothing is written then.
func (u AuditedStudentUpdates) WidenAccompaniedDaysByID(ctx context.Context, studentID int64, days []string, actorAccountID int64) (found bool, err error) {
	companion, err := u.locks.FindByIDForUpdate(ctx, studentID)
	if err != nil {
		if peoplePorts.IsRepositoryNoRows(err) {
			return false, nil
		}
		return false, err
	}
	if companion == nil {
		return false, nil
	}
	return true, u.WidenAccompaniedDays(ctx, companion, days, actorAccountID)
}

// SetLiveAbsenceFlags writes the live flags of an approved absence onto the
// child row. The row is re-read under its lock so a concurrent flag change is
// not lost. The live sick flag always takes flags.Sick; a raised one is stamped
// with at, a cleared one loses its time. flags.ClearExcused clears the live
// excused flag and its time; otherwise they stay as stored. No history is
// recorded.
func (u AuditedStudentUpdates) SetLiveAbsenceFlags(ctx context.Context, studentID int64, flags peopledirectory.LiveAbsenceFlags, at time.Time) error {
	fresh, err := u.locks.FindByIDForUpdate(ctx, studentID)
	if err != nil {
		return err
	}
	fresh.Sick, fresh.SickSince = &flags.Sick, nil
	if flags.Sick {
		fresh.SickSince = &at
	}
	if flags.ClearExcused {
		excused := false
		fresh.Excused, fresh.ExcusedSince = &excused, nil
	}
	return u.students.Update(ctx, fresh)
}

func (u AuditedStudentUpdates) write(ctx context.Context, before, after *users.Student, actorAccountID int64) error {
	if err := u.students.Update(ctx, after); err != nil {
		return err
	}
	if u.audit == nil {
		return nil
	}
	if err := u.audit.RecordChangesForActor(ctx, before, after, actorAccountID); err != nil {
		return &peopledirectory.StudentAuditError{Err: err}
	}
	return nil
}
