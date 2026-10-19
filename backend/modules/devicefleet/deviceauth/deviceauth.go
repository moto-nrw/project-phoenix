// Package deviceauth composes the device authentication middleware for the
// IoT routes. It binds the ports of the device adapter (auth/device) to the
// public Device Fleet capability, the school directory, the tenant device PIN
// setting, and the tenant runtime. Handler packages receive plain middlewares
// and never see those seams.
package deviceauth

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	device "github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/adapters/deviceauth"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Middleware is the shape every device route group mounts.
type Middleware = func(http.Handler) http.Handler

// Fleet is the part of the Device Fleet capability device authentication
// reads and writes. *devicefleet.Module satisfies it.
type Fleet interface {
	FindDeviceByAPIKey(context.Context, string) (devicefleet.Device, error)
	UpdateDeviceLastSeen(context.Context, int64, time.Time) error
}

// School is the part of a school device authentication reads.
type School struct {
	Deleted bool
}

// SchoolDirectory answers the school behind a device tenant. A school that
// does not exist is (nil, nil). The root binds it to Organisation & Tenancy.
type SchoolDirectory interface {
	FindSchool(ctx context.Context, id int64) (*School, error)
}

// Settings resolves tenant settings outside a tenant transaction. The
// settings service satisfies it.
type Settings interface {
	ResolveStringForTenant(ctx context.Context, tenantID int64, key string) (string, error)
}

// devicePINSettingKey is the Settings Platform registry key of the school's
// shared device PIN. The registry owns it; deviceauth_test.go pins it to the
// owner's public name so a renamed key fails there instead of resolving
// nothing.
const devicePINSettingKey = "security.ogs_device_pin"

// SchoolErrors classifies the school read without coupling HTTP to a SQL driver.
// Root supplies the classifier from the Device Fleet composition.
type SchoolErrors interface {
	Missing(error) bool
	Transient(error) bool
}

// Dependencies are the collaborators the device authenticators need.
type Dependencies struct {
	// Devices resolves API keys and records device activity. Required.
	Devices Fleet
	// Schools blocks devices of deleted schools. Optional: a nil directory
	// skips the check, which only test graphs use.
	Schools      SchoolDirectory
	SchoolErrors SchoolErrors
	// Settings resolves the tenant device PIN. Optional: without it only
	// FallbackPIN authenticates.
	Settings Settings
	// FallbackPIN is the OGS_DEVICE_PIN value used when no tenant PIN
	// resolves.
	FallbackPIN string
}

// Authenticators are the composed device middlewares. Both share one
// last-seen debouncer so the device routes of every resource coalesce their
// writes.
type Authenticators struct{ authenticator *device.Authenticator }

// New composes the device authenticators.
func New(deps Dependencies) *Authenticators {
	if deps.Devices == nil {
		panic("device authentication composition: the Device Fleet capability is required")
	}
	if deps.Schools != nil && deps.SchoolErrors == nil {
		panic("device authentication composition: school error classifier is required")
	}
	var schools device.SchoolLookup
	if deps.Schools != nil {
		schools = schoolLookup{schools: deps.Schools, errors: deps.SchoolErrors}
	}
	return &Authenticators{authenticator: device.NewAuthenticator(device.Dependencies{
		Devices:     fleetDirectory{fleet: deps.Devices},
		BindTenant:  bindTenant,
		Schools:     schools,
		PIN:         pinResolver(deps.Settings),
		FallbackPIN: deps.FallbackPIN,
	})}
}

// Device authenticates API key plus device PIN.
func (a *Authenticators) Device() Middleware { return a.authenticator.Device() }

// DeviceOnly authenticates the API key alone.
func (a *Authenticators) DeviceOnly() Middleware { return a.authenticator.DeviceOnly() }

// fleetDirectory serves the device port from the public capability.
// DeviceID reports the device id of the kiosk the device authenticator
// admitted on this request, or false when the request carries none. Routes
// that log or attribute the kiosk read it through this instead of the
// authenticator's context key.
func DeviceID(ctx context.Context) (string, bool) {
	authenticated := device.DeviceFromCtx(ctx)
	if authenticated == nil {
		return "", false
	}
	return authenticated.DeviceID, true
}

type fleetDirectory struct{ fleet Fleet }

func (d fleetDirectory) FindByAPIKey(ctx context.Context, apiKey string) (*device.AuthenticatedDevice, error) {
	found, err := d.fleet.FindDeviceByAPIKey(ctx, apiKey)
	if err != nil {
		return nil, err
	}
	return &device.AuthenticatedDevice{
		ID:         found.ID,
		TenantID:   found.TenantID,
		DeviceID:   found.DeviceID,
		DeviceType: found.DeviceType,
		Name:       found.Name,
		Status:     string(found.Status),
		LastSeen:   found.LastSeen,
	}, nil
}

func (d fleetDirectory) RecordLastSeen(ctx context.Context, deviceID int64, seenAt time.Time) error {
	// Each heartbeat has its own school unit, never a completed request transaction.
	return d.fleet.UpdateDeviceLastSeen(tenant.ContextWithoutTransaction(ctx), deviceID, seenAt)
}

// schoolLookup classifies the pre-tenant school-directory capability.
// Its owner supplies the explicit discovery unit before tenant middleware.
type schoolLookup struct {
	schools SchoolDirectory
	errors  SchoolErrors
}

func (l schoolLookup) IsSchoolDeleted(ctx context.Context, schoolID int64) (bool, error) {
	school, err := l.schools.FindSchool(ctx, schoolID)
	if err != nil {
		if l.errors.Missing(err) {
			return false, fmt.Errorf("%w: %w", device.ErrSchoolNotFound, err)
		}
		if l.errors.Transient(err) {
			return false, fmt.Errorf("%w: %w", device.ErrSchoolLookupUnavailable, err)
		}
		return false, err
	}
	if school == nil {
		return false, device.ErrSchoolNotFound
	}
	return school.Deleted, nil
}

// bindTenant makes the device's school the ambient tenant. Device routes do
// not use sessiontoken.TenantMiddleware, so an unusable tenant id is reported here.
func bindTenant(ctx context.Context, tenantID int64) (context.Context, error) {
	id, err := tenant.NewTenantID(tenantID)
	if err != nil {
		tenant.ObserveMissingTenant(ctx, err)
		return nil, err
	}
	return tenant.WithTenant(ctx, id), nil
}

// pinResolver reads the tenant device PIN from the settings service. It
// returns nil without a settings service, so the adapter falls back to the
// configured PIN.
func pinResolver(settings Settings) device.PINResolver {
	if settings == nil {
		return nil
	}
	return func(ctx context.Context, tenantID int64) string {
		pin, err := settings.ResolveStringForTenant(ctx, tenantID, devicePINSettingKey)
		if err != nil {
			slog.Error("failed to resolve tenant PIN from settings, falling back to env var",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
			return ""
		}
		return pin
	}
}
