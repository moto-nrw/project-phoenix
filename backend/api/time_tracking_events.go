package api

import (
	"context"
	"log/slog"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
)

// staffTimeTrackingNotifier returns the post-commit notification that
// invalidates the time-account views after a write changed a staff member's
// target working time. The root hands it to the work-time-model HTTP
// adapter, which must not reach the realtime platform itself.
func staffTimeTrackingNotifier(broadcaster realtime.Broadcaster) func(context.Context) {
	return func(ctx context.Context) {
		deliveryCompose.QueueStaffTimeTrackingChanged(ctx, broadcaster, nil)
	}
}

// timeTrackingEvents binds the time-tracking services' event port to the
// Delivery producer. The services announce a change; the producer defers the
// tenant-wide broadcast until the surrounding transaction commits.
func timeTrackingEvents(broadcaster realtime.Broadcaster) workforceCompose.EventPublisher {
	return timeTrackingEventPublisher{broadcaster: broadcaster}
}

type timeTrackingEventPublisher struct {
	broadcaster realtime.Broadcaster
}

func (e timeTrackingEventPublisher) QueueStaffTimeTrackingChanged(ctx context.Context, logger *slog.Logger) {
	deliveryCompose.QueueStaffTimeTrackingChanged(ctx, e.broadcaster, logger)
}
