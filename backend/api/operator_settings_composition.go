package api

import (
	"context"

	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/realtime"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The operator's school settings (modules/settings/compose, #2736) take two
// seams from the retained services: the tenant settings broadcast and the
// presence-mode guard's open attendance check.

// settingsChangedNotifier tells the school's open tenant tabs that a setting
// changed, so they drop their settings caches across origins. It is nil
// without a realtime hub, which disables the broadcast.
func settingsChangedNotifier(hub *realtime.Hub) settingsCompose.SettingsChangedNotifier {
	if hub == nil {
		return nil
	}
	return func(_ context.Context, tenantID int64, key string) {
		event := realtime.NewEvent(realtime.EventTenantSettingsChanged, "", realtime.EventData{Source: &key})
		_ = hub.BroadcastToTenant(tenantID, event)
	}
}

// OpenAttendanceChecker answers the presence-mode guard from Student
// Presence. Without the presence service it reports no open attendance,
// which disables the guard.
type OpenAttendanceChecker struct{ Service studentpresence.Presence }

func (c OpenAttendanceChecker) HasOpenAttendanceOn(ctx context.Context, day configModel.CalendarDate) (bool, error) {
	if c.Service == nil {
		return false, nil
	}
	value := day.UTCMidnight()
	return c.Service.HasOpenAttendanceOn(ctx, timezone.NewDate(value.Year(), value.Month(), value.Day()))
}
