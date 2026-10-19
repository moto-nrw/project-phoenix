package owners

import (
	"context"

	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
)

// SettingsChangedNotifier tells the school's open tenant tabs that a setting
// changed, so they drop their settings caches across origins. It is nil
// without a realtime hub, which disables the broadcast.
func SettingsChangedNotifier(hub *realtime.Hub) settingsCompose.SettingsChangedNotifier {
	if hub == nil {
		return nil
	}
	return func(_ context.Context, tenantID int64, key string) {
		event := realtime.NewEvent(realtime.EventTenantSettingsChanged, "", realtime.EventData{Source: &key})
		_ = hub.BroadcastToTenant(tenantID, event)
	}
}
