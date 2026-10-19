package application

import (
	context "context"

	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

type StudentAuditActorPort struct {
	peoplePorts.StudentAuditRecorder
	Actor peoplePorts.RequestAuditActor
}

func (s *StudentAuditActorPort) RecordChangesForActor(
	ctx context.Context,
	before, after *directory.StudentState,
	editedBy int64,
) error {
	return s.RecordStudentChangesForActor(ctx, StudentAuditSnapshot(before), StudentAuditSnapshot(after), editedBy)
}
func (s *StudentAuditActorPort) RecordPickupPlanForActor(
	ctx context.Context,
	studentID int64,
	before, after, result, reason string,
	editedBy int64,
) error {
	return s.RecordPickupPlan(
		ctx, studentID, before, after, result, reason, editedBy, s.actorDisplayName(ctx, editedBy))
}

// actorDisplayName resolves the editor's display name from the authenticated
// caller. A mismatched context is attributed to nobody rather than to the
// wrong person; the owner stores its own stand-in for that.
func (s *StudentAuditActorPort) actorDisplayName(ctx context.Context, editedBy int64) string {
	accountID, name := s.Actor(ctx)
	if accountID != editedBy {
		return ""
	}
	return name
}

// RecordStudentChangesForActor is the native actor-aware core; the retained
// method above only translates its legacy values into this public contract.
func (s *StudentAuditActorPort) RecordStudentChangesForActor(ctx context.Context, before, after *directory.StudentAuditSnapshot, editedBy int64) error {
	accountID, name := s.Actor(ctx)
	if before == nil || after == nil {
		return nil
	}
	if accountID != editedBy {
		name = ""
	}
	return s.RecordStudentChanges(ctx, *before, *after, editedBy, name)
}
func StudentAuditSnapshot(value *directory.StudentState) *directory.StudentAuditSnapshot {
	if value == nil {
		return nil
	}
	snapshot := value.AuditSnapshot()
	return &snapshot
}
