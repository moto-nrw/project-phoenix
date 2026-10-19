package notifications

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/delivery/devices"
)

// Notifier is the full surface the concrete service provides. It is assignable
// to Service, so consumers that only notify one audience keep their narrower
// type.
type Notifier interface {
	Service
	BatchNotifier
	SynchronousService
	DurableReceiptService
	devices.NotificationCommands
}

// NotifyEvent preserves the existing router, guards, transaction and error.
func (r *router) NotifyEvent(ctx context.Context, event devices.NotificationEvent) error {
	return r.Notify(ctx, notificationEventFromPublic(event))
}

// NotifyEventDurably preserves the existing durable receipt path.
func (r *router) NotifyEventDurably(ctx context.Context, event devices.NotificationEvent) (int, error) {
	return r.NotifyDurably(ctx, notificationEventFromPublic(event))
}

func notificationEventFromPublic(event devices.NotificationEvent) Event {
	return Event{
		Type:           event.Type,
		IdempotencyKey: event.IdempotencyKey,
		RelatedType:    event.RelatedType,
		RelatedID:      event.RelatedID,
		Audience: Audience{
			TenantID:           event.Audience.TenantID,
			Scope:              AudienceScope(event.Audience.Scope),
			GuardianAccountID:  event.Audience.GuardianAccountID,
			GuardianAccountIDs: event.Audience.GuardianAccountIDs,
			ActiveGroupID:      event.Audience.ActiveGroupID,
			StaffAccountIDs:    event.Audience.StaffAccountIDs,
			StudentIDs:         event.Audience.StudentIDs,
		},
		Priority:       event.Priority,
		Title:          event.Title,
		Body:           event.Body,
		DeepLink:       event.DeepLink,
		SchoolDeepLink: event.SchoolDeepLink,
		Data:           event.Data,
		Portal:         event.Portal,
	}
}
