package api

import (
	"context"
	"log/slog"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
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
		deliveryCompose.QueueInstanceStarted(ctx, broadcaster, logger, deliveryCompose.StartedInstance{
			ID: instance.ID, Date: instance.Date, StartTime: instance.StartTime,
			RoomID: instance.RoomID, ActiveGroupID: instance.ActiveGroupID,
		})
	}
}
