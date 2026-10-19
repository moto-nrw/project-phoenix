package owners

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	devicefleetCompose "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
)

type sessionDevices struct {
	devicefleet.SessionDeviceDirectory
	window func(context.Context) time.Duration
}

// NewSessionDeviceDirectory binds session device operations and the owner's
// online-window resolver without exposing the settings service to attendance.
func NewSessionDeviceDirectory(records devicefleet.SessionDeviceRecords, settings DeviceOnlineWindowSettings, logger *slog.Logger) presenceservice.SessionDeviceDirectory {
	return sessionDevices{SessionDeviceDirectory: devicefleetCompose.NewSessionDeviceIDs(records), window: NewOnlineWindowResolver(settings, logger)}
}

func (d sessionDevices) OnlineWindow(ctx context.Context) time.Duration {
	return d.window(ctx)
}
