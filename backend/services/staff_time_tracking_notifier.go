package services

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/delivery/application/realtimeevents"
	"github.com/moto-nrw/project-phoenix/modules/workforce/legacy/timetracking"
	"github.com/moto-nrw/project-phoenix/realtime"
)

// StaffTimeTrackingNotifier returns the post-commit notification that
// invalidates the time-account views after a write changed a staff member's
// target working time. The composition root hands it to the work-time-model
// HTTP adapter, which must not reach the realtime platform itself.
func StaffTimeTrackingNotifier(broadcaster realtime.Broadcaster) func(context.Context) {
	return func(ctx context.Context) {
		realtimeevents.QueueStaffTimeTrackingChanged(ctx, broadcaster, nil)
	}
}

// TimeTrackingEvents binds the retained time-tracking services' event port to
// the Delivery producer. The services announce a change; the producer defers
// the tenant-wide broadcast until the surrounding transaction commits.
func TimeTrackingEvents(broadcaster realtime.Broadcaster) timetracking.EventPublisher {
	return timeTrackingEvents{broadcaster: broadcaster}
}

type timeTrackingEvents struct {
	broadcaster realtime.Broadcaster
}

func (e timeTrackingEvents) QueueStaffTimeTrackingChanged(ctx context.Context, logger *slog.Logger) {
	realtimeevents.QueueStaffTimeTrackingChanged(ctx, e.broadcaster, logger)
}
