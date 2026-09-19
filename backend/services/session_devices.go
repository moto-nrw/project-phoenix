package services

import (
	"context"
	"log/slog"
	"time"

	devicefleetLegacy "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose/legacy"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/services/active"
)

type sessionDevices struct {
	*devicefleetLegacy.SessionDeviceIDs
	window func(context.Context) time.Duration
}

// NewSessionDeviceDirectory binds session device operations and the owner's
// online-window resolver without exposing the settings service to attendance.
func NewSessionDeviceDirectory(records devicefleetLegacy.SessionDeviceRecords, settings devicefleetLegacy.SettingsResolver, logger *slog.Logger) active.SessionDeviceDirectory {
	return sessionDevices{SessionDeviceIDs: &devicefleetLegacy.SessionDeviceIDs{SessionDeviceRecords: records}, window: devicefleetLegacy.NewOnlineWindowResolver(settings, logger)}
}

func (d sessionDevices) OnlineWindow(ctx context.Context) time.Duration {
	return d.window(ctx)
}
