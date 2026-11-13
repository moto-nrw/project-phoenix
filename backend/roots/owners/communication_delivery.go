package owners

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/communication"
	"github.com/moto-ogs/project-phoenix/modules/delivery/devices"
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

// BindCommunicationNotificationSeries binds the language variants of one push
// to the same router, with one device read for all of them (#5869, #5876).
// The durable receipt keeps every accepted variant with the error that ended
// the series.
func BindCommunicationNotificationSeries(source devices.NotificationSeriesCommands) communication.NotificationSeries {
	if source == nil {
		return communication.NotificationSeries{}
	}
	return communication.NotificationSeries{
		Notify: func(ctx context.Context, events []communication.NotificationEvent) (int, error) {
			return source.NotifyEvents(ctx, communicationNotificationsToDelivery(events))
		},
		NotifyDurably: func(ctx context.Context, events []communication.NotificationEvent) ([]int, error) {
			return source.NotifyEventsDurably(ctx, communicationNotificationsToDelivery(events))
		},
	}
}

func communicationNotificationsToDelivery(events []communication.NotificationEvent) []devices.NotificationEvent {
	out := make([]devices.NotificationEvent, len(events))
	for i, event := range events {
		out[i] = communicationNotificationToDelivery(event)
	}
	return out
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
