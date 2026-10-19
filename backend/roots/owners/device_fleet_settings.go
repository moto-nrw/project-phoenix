package owners

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
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

// NewOnlineWindowMinutes reads the per-tenant raw
// iot.device_online_window_minutes setting. A missing override or a lookup
// failure yields zero; Device Fleet applies its default to a non-positive
// value.
func NewOnlineWindowMinutes(settings DeviceOnlineWindowSettings, logger *slog.Logger) func(context.Context) int {
	if logger == nil {
		logger = slog.Default()
	}
	return func(ctx context.Context) int {
		return settingsCompose.ResolveIntOrDefault(ctx, settings, configModels.KeyDeviceOnlineWindowMinutes, 0, logger)
	}
}

// NewOnlineWindowResolver binds the raw setting to Device Fleet's
// online-window rule for consumers that take the resolved window.
func NewOnlineWindowResolver(settings DeviceOnlineWindowSettings, logger *slog.Logger) func(context.Context) time.Duration {
	return devicefleet.OnlineWindowFromMinutes(NewOnlineWindowMinutes(settings, logger))
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
