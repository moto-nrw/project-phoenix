package compose

import (
	"context"
	"errors"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
)

var errSettingsNotConfigured = errors.New("settings service is not configured")

type settingsResolver interface {
	ResolveString(context.Context, string) (string, error)
	ResolveBool(context.Context, string) (bool, error)
	ResolveInt(context.Context, string) (int, error)
}

// settings binds the required tenant settings service. Presence mode stays
// with the retained presence service, which validates the value.
type settings struct {
	settings settingsResolver
	active   activeSvc.Service
}

func (s settings) PresenceMode(ctx context.Context) (string, error) {
	return s.active.GetPresenceMode(ctx)
}

func (s settings) FeedbackEnabled(ctx context.Context) (bool, error) {
	if s.settings == nil {
		return false, errSettingsNotConfigured
	}
	return s.settings.ResolveBool(ctx, configModel.KeyFeedbackEnabled)
}

func (s settings) CapacityDetailsDisclosed(ctx context.Context, kind ports.CapacityKind) (bool, error) {
	if s.settings == nil {
		return false, errSettingsNotConfigured
	}
	key := configModel.KeyCheckinRoomCapacityDetailsEnabled
	if kind == ports.CapacityActivity {
		key = configModel.KeyCheckinActivityCapacityDetailsEnabled
	}
	return s.settings.ResolveBool(ctx, key)
}

// DailyCheckoutTime resolves the tenant override or registry default.
func (s settings) DailyCheckoutTime(ctx context.Context) (string, error) {
	if s.settings == nil {
		return "", errSettingsNotConfigured
	}
	return s.settings.ResolveString(ctx, configModel.KeyStudentDailyCheckoutTime)
}

func (s settings) PerStudentCheckoutEnabled(ctx context.Context) (bool, error) {
	if s.settings == nil {
		return false, nil
	}
	return s.settings.ResolveBool(ctx, configModel.KeyPerStudentCheckoutEnabled)
}

func (s settings) PerStudentCheckoutDeltaMinutes(ctx context.Context) (int, error) {
	if s.settings == nil {
		return 0, errSettingsNotConfigured
	}
	return s.settings.ResolveInt(ctx, configModel.KeyPerStudentCheckoutDeltaMinutes)
}

func (s settings) DailyCheckoutFromAllRoomsEnabled(ctx context.Context) (bool, error) {
	if s.settings == nil {
		return false, nil
	}
	return s.settings.ResolveBool(ctx, configModel.KeyCheckoutDailyFromAllRoomsEnabled)
}
