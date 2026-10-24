package planning

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// queueStaffTimeTrackingChanged announces a staff time-account change: one
// tenant-wide staff_time_tracking_changed event, sent after the surrounding
// tenant transaction commits (outside one it fires immediately). A nil
// broadcaster (bare unit fixtures) or an unbound tenant stays silent.
func queueStaffTimeTrackingChanged(ctx context.Context, broadcaster realtime.Broadcaster, logger *slog.Logger) {
	if broadcaster == nil {
		return
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID == 0 {
		return
	}
	logger = loggerOrDefault(logger)
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
