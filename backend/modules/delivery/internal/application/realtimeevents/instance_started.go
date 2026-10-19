package realtimeevents

import (
	"context"
	"log/slog"
	"strconv"

	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// TenantPublisher broadcasts an event to every client of one school.
type TenantPublisher interface {
	BroadcastToTenant(tenantID int64, event realtime.Event) error
}

// StartedInstance names the timetable instance a kiosk session started.
// RoomID 0 and a nil ActiveGroupID leave the room and the group out.
type StartedInstance struct {
	ID            int64
	Date          string
	StartTime     string
	RoomID        int64
	ActiveGroupID *int64
}

// QueueInstanceStarted announces the started instance to the school of the
// context after the surrounding transaction commits, so a rollback never
// announces an uncommitted row.
func QueueInstanceStarted(ctx context.Context, broadcaster TenantPublisher, logger *slog.Logger, instance StartedInstance) {
	if broadcaster == nil {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}
	id := strconv.FormatInt(instance.ID, 10)
	data := realtime.EventData{InstanceID: &id, InstanceDate: &instance.Date, InstanceStartTime: &instance.StartTime}
	if instance.RoomID > 0 {
		room := strconv.FormatInt(instance.RoomID, 10)
		data.RoomID = &room
	}
	groupID := ""
	if instance.ActiveGroupID != nil {
		groupID = strconv.FormatInt(*instance.ActiveGroupID, 10)
	}
	event := realtime.NewEvent(realtime.EventInstanceStarted, groupID, data)
	tenantID := tenant.FromContext(ctx)
	tenant.RegisterAfterCommit(ctx, func() {
		if err := broadcaster.BroadcastToTenant(tenantID, event); err != nil {
			logger.WarnContext(ctx, "failed to broadcast mirrored timetable instance",
				slog.String("event_type", string(realtime.EventInstanceStarted)),
				slog.Int64("instance_id", instance.ID),
				slog.String("error", err.Error()),
			)
		}
	})
}
