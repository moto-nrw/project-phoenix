package realtimeevents

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// QueueActiveSupervisionChanged refreshes tenant-wide live-supervision views
// after the surrounding transaction commits.
func QueueActiveSupervisionChanged(ctx context.Context, broadcaster realtime.Broadcaster, logger *slog.Logger, activeGroupID int64, reason string) {
	if broadcaster == nil || activeGroupID <= 0 {
		return
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID == 0 {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}
	id := fmt.Sprintf("%d", activeGroupID)
	tenant.RegisterAfterCommit(ctx, func() {
		event := realtime.NewEvent(realtime.EventActiveSupervisionChanged, id, realtime.EventData{Reason: &reason})
		if err := broadcaster.BroadcastToTenant(tenantID, event); err != nil {
			logger.Warn("SSE active supervision broadcast failed",
				slog.String("reason", reason),
				slog.Int64("tenant_id", tenantID),
				slog.Int64("active_group_id", activeGroupID),
				slog.String("error", err.Error()),
			)
		}
	})
}
