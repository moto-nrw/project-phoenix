package compose

import (
	"context"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// The contracts below are what the retained services call when they mutate an
// audited student field; they left services/users with the person and student
// services (#3753). The rules behind them — which fields are tracked,
// how a change reads and where it is stored — belong to the People Directory
// owner and the Audit Platform; #3349 moved them there. The composition root
// binds an implementation (api.NewStudentAuditFor). Reading
// the trail back is the owner's StudentAuditQuery, not part of this contract.

type studentAuditActorPort struct {
	recorder peoplePorts.StudentAuditRecorder
	actor    peoplePorts.RequestAuditActor
}

// NewStudentAuditService resolves the authenticated editor and hands every
// recorded change to the owner capability behind recorder.
func NewStudentAuditService(actor peoplePorts.RequestAuditActor, recorder peoplePorts.StudentAuditRecorder) peoplePorts.StudentAuditService {
	if actor == nil {
		panic("request audit actor is required")
	}
	return &studentAuditActorPort{recorder: recorder, actor: actor}
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
	return s.recorder.RecordChanges(ctx, before, after, editedBy, s.actorDisplayName(ctx, editedBy))
}

func (s *studentAuditActorPort) RecordPickupPlanForActor(
	ctx context.Context,
	studentID int64,
	before, after, result, reason string,
	editedBy int64,
) error {
	return s.recorder.RecordPickupPlan(
		ctx, studentID, before, after, result, reason, editedBy, s.actorDisplayName(ctx, editedBy))
}

func (s *studentAuditActorPort) RecordSystemStatusChange(
	ctx context.Context,
	studentID int64,
	before, after userModels.StudentStatus,
) error {
	return s.recorder.RecordSystemStatusChange(ctx, studentID, before, after)
}

// actorDisplayName resolves the editor's display name from the authenticated
// caller. A mismatched context is attributed to nobody rather than to the
// wrong person; the owner stores its own stand-in for that.
func (s *studentAuditActorPort) actorDisplayName(ctx context.Context, editedBy int64) string {
	accountID, name := s.actor(ctx)
	if accountID != editedBy {
		return ""
	}
	return name
}
