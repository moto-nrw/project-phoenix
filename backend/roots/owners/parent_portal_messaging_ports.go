package owners

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	settingsValues "github.com/moto-nrw/project-phoenix/modules/settings"
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
// BindParentLeadershipChoice binds the parents portal's leadership option to
// Communication's rule over the school's switch and its leadership persons
// (#5200). It runs in the child's tenant transaction.
func BindParentLeadershipChoice(settings settingsValues.ParentMessagingValueSource, leaders communication.ParentMessageLeaderDirectory) func(context.Context) (parentportal.LeadershipOption, error) {
	values := settingsValues.NewParentMessagingValues(settings)
	return func(ctx context.Context) (parentportal.LeadershipOption, error) {
		choice, err := communication.ResolveParentMessageLeadershipChoice(ctx, values, leaders)
		if err != nil {
			return parentportal.LeadershipOption{}, err
		}
		return parentportal.LeadershipOption{Enabled: choice.Enabled, Names: choice.ShortNames()}, nil
	}
}

func BindParentMessagingGate(settings communication.ParentMessagingSettings, logger *slog.Logger) parentportal.MessagingGate {
	return func(ctx context.Context, tenantID int64) bool {
		return communication.ParentMessagingEnabledForTenant(ctx, settings, tenantID, logger)
	}
}
