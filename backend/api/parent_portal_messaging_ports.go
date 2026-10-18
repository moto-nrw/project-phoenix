package api

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// BindParentChildEvents translates the workflow's pills to Communication's
// public event value. Nil preserves the workflow's optional notification path.
func BindParentChildEvents(source communication.ParentChildEvents) parentportal.ChildEvents {
	if source == nil {
		return nil
	}
	return parentChildEvents{source: source}
}

type parentChildEvent = parentportal.ChildEvent

type parentChildEvents struct {
	source communication.ParentChildEvents
}

func (e parentChildEvents) EmitChildEvent(ctx context.Context, tenantID, studentID, accountID int64, event parentChildEvent) error {
	return e.source.EmitParentChildEvent(ctx, tenantID, studentID, accountID, communication.ParentChildEvent{
		EventType: event.EventType, ActorKind: event.ActorKind, ActorAccountID: event.ActorAccountID,
		Body: event.Body, RequestType: event.RequestType, RequestStatus: event.RequestStatus,
		RefTable: event.RefTable, RefID: event.RefID,
	})
}

func (e parentChildEvents) BroadcastChildUpdateToGuardians(tenantID, studentID int64) {
	e.source.BroadcastChildUpdateToGuardians(tenantID, studentID)
}

// BindParentMessagingGate keeps the shared Communication policy and the
// caller's explicit-tenant settings reader, including its failure direction.
func BindParentMessagingGate(settings communication.ParentMessagingSettings, logger *slog.Logger) parentportal.MessagingGate {
	return func(ctx context.Context, tenantID int64) bool {
		return communication.ParentMessagingEnabledForTenant(ctx, settings, tenantID, logger)
	}
}
