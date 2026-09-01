package realtimeevents

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// QueueStaffTimeTrackingChanged emits a tenant-wide invalidation after the
// surrounding transaction commits.
func QueueStaffTimeTrackingChanged(ctx context.Context, broadcaster realtime.Broadcaster, logger *slog.Logger) {
	if broadcaster == nil {
		return
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID == 0 {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}
	tenant.RegisterAfterCommit(ctx, func() {
		event := realtime.NewEvent(realtime.EventStaffTimeTrackingChanged, "", realtime.EventData{})
		if err := broadcaster.BroadcastToTenant(tenantID, event); err != nil {
			logger.Warn("SSE staff time-tracking broadcast failed",
				slog.String("error", err.Error()),
				slog.Int64("tenant_id", tenantID),
			)
		}
	})
}
