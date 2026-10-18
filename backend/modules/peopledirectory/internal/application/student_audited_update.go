package application

import (
	"context"
	"errors"
	"fmt"

	users "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// StudentRowUpdate is the retained child row write an audited update runs.
type StudentRowUpdate interface {
	Update(ctx context.Context, student *users.Student) error
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
	audit    StudentChangeActorRecorder
}

// NewAuditedStudentUpdates binds the command. audit may be nil.
func NewAuditedStudentUpdates(students StudentRowUpdate, audit StudentChangeActorRecorder) AuditedStudentUpdates {
	return AuditedStudentUpdates{students: students, audit: audit}
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
