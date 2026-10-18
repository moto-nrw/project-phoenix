package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

var errSettingsNotConfigured = errors.New("settings service is not configured")

// ScanSettings are the tenant settings the scans read; the root binds them
// to the Settings Platform.
type ScanSettings interface {
	FeedbackEnabled(ctx context.Context) (bool, error)
	RoomCapacityDetailsDisclosed(ctx context.Context) (bool, error)
	ActivityCapacityDetailsDisclosed(ctx context.Context) (bool, error)
	DailyCheckoutTime(ctx context.Context) (string, error)
	PerStudentCheckoutEnabled(ctx context.Context) (bool, error)
	PerStudentCheckoutDeltaMinutes(ctx context.Context) (int, error)
	DailyCheckoutFromAllRoomsEnabled(ctx context.Context) (bool, error)
}

// settings binds the required tenant settings service. Presence mode stays
// with the retained presence service, which validates the value.
type settings struct {
	settings ScanSettings
	active   studentpresence.Presence
}

func (s settings) PresenceMode(ctx context.Context) (string, error) {
	return s.active.GetPresenceMode(ctx)
}

func (s settings) FeedbackEnabled(ctx context.Context) (bool, error) {
	if s.settings == nil {
		return false, errSettingsNotConfigured
	}
	return s.settings.FeedbackEnabled(ctx)
}

func (s settings) CapacityDetailsDisclosed(ctx context.Context, kind ports.CapacityKind) (bool, error) {
	if s.settings == nil {
		return false, errSettingsNotConfigured
	}
	if kind == ports.CapacityActivity {
		return s.settings.ActivityCapacityDetailsDisclosed(ctx)
	}
	return s.settings.RoomCapacityDetailsDisclosed(ctx)
}

// DailyCheckoutTime resolves the tenant override or registry default.
func (s settings) DailyCheckoutTime(ctx context.Context) (string, error) {
	if s.settings == nil {
		return "", errSettingsNotConfigured
	}
	return s.settings.DailyCheckoutTime(ctx)
}

func (s settings) PerStudentCheckoutEnabled(ctx context.Context) (bool, error) {
	if s.settings == nil {
		return false, nil
	}
	return s.settings.PerStudentCheckoutEnabled(ctx)
}

func (s settings) PerStudentCheckoutDeltaMinutes(ctx context.Context) (int, error) {
	if s.settings == nil {
		return 0, errSettingsNotConfigured
	}
	return s.settings.PerStudentCheckoutDeltaMinutes(ctx)
}

func (s settings) DailyCheckoutFromAllRoomsEnabled(ctx context.Context) (bool, error) {
	if s.settings == nil {
		return false, nil
	}
	return s.settings.DailyCheckoutFromAllRoomsEnabled(ctx)
}
