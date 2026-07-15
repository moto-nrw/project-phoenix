package schedule

import (
	"context"
	"fmt"
	"log/slog"

	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// QueueActivityUpdates queues one activity_update per affected active group.
// The event is a refetch trigger, so it carries only the instance slot identity.
func (s *instanceService) QueueActivityUpdates(ctx context.Context, touched map[int64]*scheduleModel.ActivityInstance) {
	if s.deps.Broadcaster == nil || len(touched) == 0 {
		return
	}
	tenantID := tenant.FromContext(ctx)
	for activeGroupID, instance := range touched {
		if instance == nil {
			continue
		}
		activeGroupIDStr := fmt.Sprintf("%d", activeGroupID)
		instanceIDStr := fmt.Sprintf("%d", instance.ID)
		instanceDate := instance.Date.String()
		instanceStart := instance.StartTime.Format("15:04:05")
		event := realtime.NewEvent(realtime.EventActivityUpdate, activeGroupIDStr, realtime.EventData{
			InstanceID:        &instanceIDStr,
			InstanceDate:      &instanceDate,
			InstanceStartTime: &instanceStart,
		})
		tenant.RegisterAfterCommit(ctx, func() {
			if err := s.deps.Broadcaster.BroadcastToGroup(tenantID, activeGroupIDStr, event); err != nil {
				s.getLogger().Warn("SSE activity update broadcast failed",
					slog.String("active_group_id", activeGroupIDStr),
					slog.String("error", err.Error()),
				)
			}
		})
	}
}
