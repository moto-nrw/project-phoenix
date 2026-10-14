package compose

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/delivery/application/realtimeevents"
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
