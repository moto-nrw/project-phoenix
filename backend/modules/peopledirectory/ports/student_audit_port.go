package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// StudentChangeRecorder is the narrow write contract used by services that
// mutate audited student fields.
type StudentChangeRecorder interface {
	// RecordChanges diffs the before/after student snapshots and appends one
	// audit row per changed tracked field. A no-op when nothing tracked changed.
	RecordChanges(ctx context.Context, before, after *peopledirectory.StudentState, editedBy int64, editedByName string) error

	// RecordChangesForActor derives the display name from the authenticated
	// actor in ctx. The explicit account ID prevents a mismatched context from
	// attributing an edit to the wrong person.
	RecordChangesForActor(ctx context.Context, before, after *peopledirectory.StudentState, editedBy int64) error
}

// StudentPickupPlanRecorder is the narrow append-only audit seam used by the
// permanent pickup-time adjustment coordinator.
type StudentPickupPlanRecorder interface {
	RecordPickupPlanForActor(
		ctx context.Context,
		studentID int64,
		before, after, result, reason string,
		editedBy int64,
	) error
}

type StudentAuditService interface {
	peopledirectory.StudentAuditActorCommand
	StudentChangeRecorder
	StudentPickupPlanRecorder

	// RecordSystemStatusChange records an automated lifecycle transition.
	RecordSystemStatusChange(ctx context.Context, studentID int64, before, after string) error
}

// StudentAuditRecorder is the name-explicit form of the contract above: it
// takes the editor's display name instead of deriving it from the request.
// The composition root binds the People Directory owner capability to it
// (database/repositories.NewStudentAudit); this package only resolves who the
// authenticated caller is, which is not a directory decision.
type StudentAuditRecorder interface {
	RecordStudentChanges(context.Context, peopledirectory.StudentAuditSnapshot, peopledirectory.StudentAuditSnapshot, int64, string) error
	RecordChanges(ctx context.Context, before, after *peopledirectory.StudentState, editedBy int64, editedByName string) error
	RecordPickupPlan(ctx context.Context, studentID int64, before, after, result, reason string, editedBy int64, editedByName string) error
	RecordSystemStatusChange(ctx context.Context, studentID int64, before, after string) error
}

// RequestAuditActor supplies the authenticated editor, without exposing the
// transport's token or context representation to People Directory.
type RequestAuditActor func(context.Context) (accountID int64, displayName string)
