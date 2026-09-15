package services

import (
	"context"
	"log/slog"
	"strconv"

	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// KioskMirroredSession is the composed mirror notification.
type KioskMirroredSession = devicescanCompose.MirroredSession

// KioskMirrorPublisher retains the instance-started event and queues it on
// the request transaction, so a rollback never announces an uncommitted row.
func KioskMirrorPublisher(broadcaster realtime.Broadcaster, logger *slog.Logger) func(context.Context, KioskMirroredSession) {
	return func(ctx context.Context, instance KioskMirroredSession) {
		if broadcaster == nil {
			return
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
}
