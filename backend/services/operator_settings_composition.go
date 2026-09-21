package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/services/active"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/config"
)

// The operator's school settings (modules/settings/compose, #2736) take two
// seams from the retained services: the tenant settings broadcast and the
// presence-mode guard's open attendance check.

// SettingsChangedNotifier tells the school's open tenant tabs that a setting
// changed, so they drop their settings caches across origins. It is nil
// without a realtime hub, which disables the broadcast.
func (f *Factory) SettingsChangedNotifier() config.SettingsChangedNotifier {
	if f.RealtimeHub == nil {
		return nil
	}
	hub := f.RealtimeHub
	return func(_ context.Context, tenantID int64, key string) {
		event := realtime.NewEvent(realtime.EventTenantSettingsChanged, "", realtime.EventData{Source: &key})
		_ = hub.BroadcastToTenant(tenantID, event)
	}
}

// OpenAttendanceChecker answers the presence-mode guard from Student
// Presence. Without the presence service it reports no open attendance,
// which disables the guard.
func (f *Factory) OpenAttendanceChecker() config.OpenAttendanceChecker {
	return openAttendanceChecker{service: f.Active}
}

type openAttendanceChecker struct{ service active.Service }

func (c openAttendanceChecker) HasOpenAttendanceOn(ctx context.Context, day configModel.CalendarDate) (bool, error) {
	if c.service == nil {
		return false, nil
	}
	value := day.UTCMidnight()
	return c.service.HasOpenAttendanceOn(ctx, timezone.NewDate(value.Year(), value.Month(), value.Day()))
}
