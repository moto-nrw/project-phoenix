package active

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// broadcastActiveSupervisionChanged sends the generic tenant-wide refresh
// signal consumed by the active-supervisions page. Specific events still carry
// their detailed semantics; this adapter event gives every client one stable
// cache invalidation path regardless of the write source.
func (s *service) broadcastActiveSupervisionChanged(ctx context.Context, activeGroupID, studentID, reason string) {
	if s.Broadcaster == nil {
		return
	}

	data := realtime.EventData{
		Reason: &reason,
	}
	if studentID != "" {
		data.StudentID = &studentID
	}

	tenantID := tenant.FromContext(ctx)
	event := realtime.NewEvent(realtime.EventActiveSupervisionChanged, activeGroupID, data)
	if err := s.Broadcaster.BroadcastToTenant(tenantID, event); err != nil {
		s.getLogger().Warn("SSE active supervision broadcast failed",
			slog.String("error", err.Error()),
			slog.String("active_group_id", activeGroupID),
			slog.String("reason", reason),
			slog.Int64("tenant_id", tenantID),
		)
	}
}
