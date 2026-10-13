package services

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/delivery/application/realtimeevents"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/realtime"
	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
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
		realtimeevents.QueueInstanceStarted(ctx, broadcaster, logger, realtimeevents.StartedInstance{
			ID: instance.ID, Date: instance.Date, StartTime: instance.StartTime,
			RoomID: instance.RoomID, ActiveGroupID: instance.ActiveGroupID,
		})
	}
}
