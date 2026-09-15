package active

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/delivery/application/realtimeevents"
)

func queueStaffTimeTrackingChanged(ctx context.Context, broadcaster EventPublisher, logger *slog.Logger) {
	realtimeevents.QueueStaffTimeTrackingChanged(ctx, broadcaster, logger)
}
