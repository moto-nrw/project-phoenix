package active

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/delivery/application/realtimeevents"
	"github.com/moto-nrw/project-phoenix/realtime"
)

func queueStaffTimeTrackingChanged(ctx context.Context, broadcaster realtime.Broadcaster, logger *slog.Logger) {
	realtimeevents.QueueStaffTimeTrackingChanged(ctx, broadcaster, logger)
}
