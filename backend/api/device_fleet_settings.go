package api

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	devicefleetCompose "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// DeviceOnlineWindowSettings is the subset of the settings service the
// device online window needs.
type DeviceOnlineWindowSettings interface {
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	ResolveInt(ctx context.Context, key string) (int, error)
}

// NewOnlineWindowResolver reads the per-tenant
// iot.device_online_window_minutes setting. A missing override, a
// non-positive value, or a lookup failure yields zero, and Device Fleet then
// applies its own default.
func NewOnlineWindowResolver(settings DeviceOnlineWindowSettings, logger *slog.Logger) func(context.Context) time.Duration {
	if logger == nil {
		logger = slog.Default()
	}
	return func(ctx context.Context) time.Duration {
		minutes := settingsCompose.ResolveIntOrDefault(ctx, settings, configModels.KeyDeviceOnlineWindowMinutes, 0, logger)
		if minutes <= 0 {
			return 0
		}
		return time.Duration(minutes) * time.Minute
	}
}

// DisplaySchoolDirectory reads the display facts of one school. Organisation
// & Tenancy owns the row; the root binds this port to its capability.
type DisplaySchoolDirectory interface {
	// FindDisplaySchool reports the school's name and lifecycle state; found
	// is false when the school does not exist.
	FindDisplaySchool(ctx context.Context, tenantID int64) (school devicefleetCompose.School, found bool, err error)
}

// displayTenantSettings resolves one tenant's display toggle.
type displayTenantSettings interface {
	ResolveBoolForTenant(ctx context.Context, tenantID int64, key string) (bool, error)
}

type displayTenantFacts struct {
	schools  DisplaySchoolDirectory
	settings displayTenantSettings
}

// NewDisplayTenantFacts answers the pre-tenant questions a display token
// raises over the school directory and the settings.
func NewDisplayTenantFacts(schools DisplaySchoolDirectory, settings displayTenantSettings) devicefleetCompose.TenantFacts {
	if schools == nil || settings == nil {
		panic("devicefleet tenant facts: all dependencies are required")
	}
	return displayTenantFacts{schools: schools, settings: settings}
}

// School reports the tenant's name and lifecycle state. A school that cannot
// be found is reported as an unknown display, so a dead link never reveals
// whether the tenant ever existed.
func (f displayTenantFacts) School(ctx context.Context, tenantID int64) (devicefleetCompose.School, error) {
	school, found, err := f.schools.FindDisplaySchool(ctx, tenantID)
	if err != nil {
		return devicefleetCompose.School{}, fmt.Errorf("failed to load display school: %w", err)
	}
	if !found {
		return devicefleetCompose.School{}, devicefleetCompose.ErrDisplayNotFound
	}
	return school, nil
}

// DisplayEnabled resolves the opt-in display.enabled toggle for one tenant.
func (f displayTenantFacts) DisplayEnabled(ctx context.Context, tenantID int64) (bool, error) {
	return f.settings.ResolveBoolForTenant(ctx, tenantID, configModels.KeyDisplayEnabled)
}

// DeviceAPIKeys mints device API keys through Security Runtime, which owns the
// key format and its entropy.
type DeviceAPIKeys struct{}

func (DeviceAPIKeys) Mint() (string, error) { return securityruntime.DeviceAPIKey() }
