package compose

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/application/realtimeevents"
	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/moto-nrw/project-phoenix/realtime"
)

func NewRealtimeHub(logger *slog.Logger) *realtime.Hub {
	return realtime.NewHub(logger, realtime.Observers{
		Connection: observability.RecordSSEConnection,
		Broadcast:  observability.RecordSSEBroadcast,
	})
}

// QueueStaffTimeTrackingChanged emits the tenant-wide time-tracking
// invalidation after the surrounding transaction commits. The root binds it
// to the Workforce event port; the producer stays Delivery's.
func QueueStaffTimeTrackingChanged(ctx context.Context, broadcaster realtime.Broadcaster, logger *slog.Logger) {
	realtimeevents.QueueStaffTimeTrackingChanged(ctx, broadcaster, logger)
}

// StartedInstance names the timetable instance a kiosk session started.
type StartedInstance = realtimeevents.StartedInstance

// QueueInstanceStarted announces a started timetable instance to its school
// after the surrounding transaction commits. The root binds it to the kiosk
// session mirror.
func QueueInstanceStarted(ctx context.Context, broadcaster realtime.Broadcaster, logger *slog.Logger, instance StartedInstance) {
	realtimeevents.QueueInstanceStarted(ctx, broadcaster, logger, instance)
}

// QueueArrivalScheduleChanged announces an arrival schedule change after the
// surrounding transaction commits. The root binds it to the class day's
// announcer.
func QueueArrivalScheduleChanged(ctx context.Context, broadcaster interface{ BroadcastToAll(realtime.Event) error }, logger *slog.Logger) {
	realtimeevents.QueueArrivalScheduleChanged(ctx, broadcaster, logger)
}

// NewGroupEvents binds School Structure's post-commit invalidations.
func NewGroupEvents(broadcaster RealtimePublisher, logger *slog.Logger) realtimeevents.GroupEvents {
	return realtimeevents.GroupEvents{Broadcaster: broadcaster, Logger: logger}
}
