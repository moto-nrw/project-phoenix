package users

import (
	"context"
	"strings"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
)

// The contracts below are what the retained services call when they mutate an
// audited student field. The rules behind them — which fields are tracked,
// how a change reads and where it is stored — belong to the People Directory
// owner and the Audit Platform; #3349 moved them there. The composition root
// binds an implementation (database/repositories.NewStudentAuditFor).

// StudentChangeRecorder is the narrow write contract used by services that
// mutate audited student fields.
type StudentChangeRecorder interface {
	// RecordChanges diffs the before/after student snapshots and appends one
	// audit row per changed tracked field. A no-op when nothing tracked changed.
	RecordChanges(ctx context.Context, before, after *userModels.Student, editedBy int64, editedByName string) error

	// RecordChangesForActor derives the display name from the authenticated
	// actor in ctx. The explicit account ID prevents a mismatched context from
	// attributing an edit to the wrong person.
	RecordChangesForActor(ctx context.Context, before, after *userModels.Student, editedBy int64) error
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
	StudentChangeRecorder
	StudentPickupPlanRecorder

	// RecordSystemStatusChange records an automated lifecycle transition.
	RecordSystemStatusChange(ctx context.Context, studentID int64, before, after userModels.StudentStatus) error

	// GetChangeHistory returns the student's change history, newest first.
	GetChangeHistory(ctx context.Context, studentID int64) ([]*auditModels.StudentFieldEdit, error)
}

// StudentAuditRecorder is the name-explicit form of the contract above: it
// takes the editor's display name instead of deriving it from the request.
// The composition root binds the People Directory owner capability to it
// (database/repositories.NewStudentAudit); this package only resolves who the
// authenticated caller is, which is not a directory decision.
type StudentAuditRecorder interface {
	RecordChanges(ctx context.Context, before, after *userModels.Student, editedBy int64, editedByName string) error
	RecordPickupPlan(ctx context.Context, studentID int64, before, after, result, reason string, editedBy int64, editedByName string) error
	RecordSystemStatusChange(ctx context.Context, studentID int64, before, after userModels.StudentStatus) error
	GetChangeHistory(ctx context.Context, studentID int64) ([]*auditModels.StudentFieldEdit, error)
}

type studentAuditActorPort struct{ recorder StudentAuditRecorder }

// NewStudentAuditService resolves the authenticated editor and hands every
// recorded change to the owner capability behind recorder.
func NewStudentAuditService(recorder StudentAuditRecorder) StudentAuditService {
	return &studentAuditActorPort{recorder: recorder}
}

func (s *studentAuditActorPort) RecordChanges(
	ctx context.Context,
	before, after *userModels.Student,
	editedBy int64,
	editedByName string,
) error {
	return s.recorder.RecordChanges(ctx, before, after, editedBy, editedByName)
}

func (s *studentAuditActorPort) RecordChangesForActor(
	ctx context.Context,
	before, after *userModels.Student,
	editedBy int64,
) error {
	return s.recorder.RecordChanges(ctx, before, after, editedBy, actorDisplayName(ctx, editedBy))
}

func (s *studentAuditActorPort) RecordPickupPlanForActor(
	ctx context.Context,
	studentID int64,
	before, after, result, reason string,
	editedBy int64,
) error {
	return s.recorder.RecordPickupPlan(
		ctx, studentID, before, after, result, reason, editedBy, actorDisplayName(ctx, editedBy))
}

func (s *studentAuditActorPort) RecordSystemStatusChange(
	ctx context.Context,
	studentID int64,
	before, after userModels.StudentStatus,
) error {
	return s.recorder.RecordSystemStatusChange(ctx, studentID, before, after)
}

func (s *studentAuditActorPort) GetChangeHistory(
	ctx context.Context,
	studentID int64,
) ([]*auditModels.StudentFieldEdit, error) {
	return s.recorder.GetChangeHistory(ctx, studentID)
}

// actorDisplayName resolves the editor's display name from the authenticated
// caller. A mismatched context is attributed to nobody rather than to the
// wrong person; the owner stores its own stand-in for that.
func actorDisplayName(ctx context.Context, editedBy int64) string {
	claims := jwt.ClaimsFromCtx(ctx)
	if int64(claims.ID) != editedBy {
		return ""
	}
	name := strings.TrimSpace(claims.FirstName + " " + claims.LastName)
	if name == "" {
		name = strings.TrimSpace(claims.Username)
	}
	return name
}
