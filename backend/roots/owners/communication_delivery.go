package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/delivery/devices"
)

// BindCommunicationNotificationSender preserves an optional producer and binds
// its complete event to the same public Delivery router.
func BindCommunicationNotificationSender(source devices.NotificationCommands) func(context.Context, communication.NotificationEvent) error {
	if source == nil {
		return nil
	}
	return func(ctx context.Context, event communication.NotificationEvent) error {
		return source.NotifyEvent(ctx, communicationNotificationToDelivery(event))
	}
}

// BindCommunicationDurableNotificationSender keeps the original receipt and
// error together, including partial receipts on failure.
func BindCommunicationDurableNotificationSender(source devices.NotificationCommands) func(context.Context, communication.NotificationEvent) (int, error) {
	if source == nil {
		return nil
	}
	return func(ctx context.Context, event communication.NotificationEvent) (int, error) {
		return source.NotifyEventDurably(ctx, communicationNotificationToDelivery(event))
	}
}

func communicationNotificationToDelivery(event communication.NotificationEvent) devices.NotificationEvent {
	return devices.NotificationEvent{
		Type: event.Type, IdempotencyKey: event.IdempotencyKey,
		RelatedType: event.RelatedType, RelatedID: event.RelatedID,
		Audience: devices.NotificationAudience(event.Audience),
		Priority: event.Priority, Title: event.Title, Body: event.Body,
		DeepLink: event.DeepLink, SchoolDeepLink: event.SchoolDeepLink,
		Data: event.Data, Portal: event.Portal,
	}
}
