package owners

import (
	"context"
	"log/slog"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
)

// TimeTrackingEvents binds the time-tracking services' event port to the
// Delivery producer. The services announce a change; the producer defers the
// tenant-wide broadcast until the surrounding transaction commits.
func TimeTrackingEvents(broadcaster realtime.Broadcaster) workforceCompose.EventPublisher {
	return timeTrackingEventPublisher{broadcaster: broadcaster}
}

type timeTrackingEventPublisher struct {
	broadcaster realtime.Broadcaster
}

func (e timeTrackingEventPublisher) QueueStaffTimeTrackingChanged(ctx context.Context, logger *slog.Logger) {
	deliveryCompose.QueueStaffTimeTrackingChanged(ctx, e.broadcaster, logger)
}
