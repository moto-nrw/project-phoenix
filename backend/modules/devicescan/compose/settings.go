package compose

import (
	"context"
	"errors"
	"log/slog"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
)

var errSettingsNotConfigured = errors.New("settings service is not configured")

// settings binds the tenant settings the scans read. A nil settings service
// answers the registry defaults the flows fall back to; the presence mode
// stays with the retained presence service, which validates the value.
type settings struct {
	settings configSvc.SettingsService
	active   activeSvc.Service
	fallback string
	logger   *slog.Logger
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

// DailyCheckoutTime resolves the raw gate: tenant override, then the
// STUDENT_DAILY_CHECKOUT_TIME process value, then empty (no gate).
func (s settings) DailyCheckoutTime(ctx context.Context) string {
	return configSvc.ResolveStringOrDefault(ctx, s.settings, configModel.KeyStudentDailyCheckoutTime, s.fallback, s.logger)
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
