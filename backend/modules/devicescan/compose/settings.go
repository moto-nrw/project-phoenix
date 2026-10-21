package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/application"

	"github.com/moto-nrw/project-phoenix/modules/devicescan/ports"
)

var errSettingsNotConfigured = application.ErrSettingsNotConfigured

// ScanSettings are the tenant settings the scans read; the root binds them
// to the Settings Platform.
type ScanSettings interface{ application.ScanSettings }

// settings binds the required tenant settings service. Presence mode stays
// with the retained presence service, which validates the value; its
// GetPresenceMode is promoted unchanged.
type settings struct {
	settings ScanSettings
	presenceModes
}

type presenceModes interface {
	GetPresenceMode(ctx context.Context) (string, error)
}

func (s settings) FeedbackEnabled(ctx context.Context) (bool, error) {
	return application.FeedbackEnabledBinding(ctx, s.settings)
}

func (s settings) CapacityDetailsDisclosed(ctx context.Context, kind ports.CapacityKind) (bool, error) {
	return application.CapacityDetailsDisclosedBinding(ctx, s.settings, kind)
}

// DailyCheckoutTime resolves the tenant override or registry default.
func (s settings) DailyCheckoutTime(ctx context.Context) (string, error) {
	return application.DailyCheckoutTimeBinding(ctx, s.settings)
}

func (s settings) PerStudentCheckoutEnabled(ctx context.Context) (bool, error) {
	return application.PerStudentCheckoutEnabledBinding(ctx, s.settings)
}

func (s settings) PerStudentCheckoutDeltaMinutes(ctx context.Context) (int, error) {
	return application.PerStudentCheckoutDeltaMinutesBinding(ctx, s.settings)
}

func (s settings) DailyCheckoutFromAllRoomsEnabled(ctx context.Context) (bool, error) {
	return application.DailyCheckoutFromAllRoomsEnabledBinding(ctx, s.settings)
}
