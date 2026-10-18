package messaging

import (
	"context"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/care"
)

// EmitSelfServicePill posts the chat pill mirroring a parent self-service
// action (sick note, one-day pickup change). It MUST be scheduled from a
// tenant.RegisterAfterCommit callback: the emitter opens its own detached
// tenant transaction and is best-effort, so a pill failure can never roll
// back the sick note / pickup change (nor vice versa).
func (s *Service) EmitSelfServicePill(ctx context.Context, tenantID, studentID, accountID int64, eventType, body, refTable string, refID *int64) error {
	if s.Emitter == nil {
		return nil
	}
	return s.Emitter.EmitChildEvent(ctx, tenantID, studentID, accountID, care.ChildEvent{
		EventType:      eventType,
		ActorKind:      ParentMessageSenderGuardian,
		ActorAccountID: accountID,
		Body:           body,
		RefTable:       refTable,
		RefID:          refID,
	})
}

// WakeChildGuardians fans a message-INDEPENDENT parent_child_updated SSE event
// to EVERY guardian of the child, so a SECOND guardian's already-open parents-
// app tab refetches the child's care state after THIS guardian's self-service
// write (sick note, care exception). emitSelfServicePill only appends a pill to
// the ACTING guardian's own thread and wakes that guardian; a co-guardian would
// otherwise keep showing a stale "Heute" pickup time or presence until they
// refocus or reload (#1725 review). Like emitSelfServicePill it MUST be
// scheduled from a tenant.RegisterAfterCommit callback: BroadcastChildUpdate-
// ToGuardians opens its own detached tenant transaction to read the guardian
// list, so a woken client never reads the pre-commit snapshot. A nil emitter is
// a safe no-op.
func (s *Service) WakeChildGuardians(tenantID, studentID int64) {
	if s.Emitter == nil {
		return
	}
	s.Emitter.BroadcastChildUpdateToGuardians(tenantID, studentID)
}
