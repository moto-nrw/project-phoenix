// Package deviceauth composes the device authentication middleware for the
// IoT routes. It binds the ports of the device adapter (auth/device) to the
// public Device Fleet capability, the school directory, the retained staff
// PIN verification, the tenant device PIN setting, and the tenant runtime.
// Handler packages receive plain middlewares and never see those seams.
package deviceauth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/device"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun/driver/pgdriver"
)

// Middleware is the shape every device route group mounts.
type Middleware = func(http.Handler) http.Handler

// Fleet is the part of the Device Fleet capability device authentication
// reads and writes. *devicefleet.Module satisfies it.
type Fleet interface {
	FindDeviceByAPIKey(context.Context, string) (devicefleet.Device, error)
	UpdateDeviceLastSeen(context.Context, int64, time.Time) error
}

// SchoolDirectory answers the school row behind a device tenant. The
// retained school service satisfies it.
type SchoolDirectory interface {
	GetSchoolByID(ctx context.Context, id int64) (*platform.School, error)
}

// Settings resolves tenant settings outside a tenant transaction. The
// settings service satisfies it.
type Settings interface {
	ResolveStringForTenant(ctx context.Context, tenantID int64, key string) (string, error)
}

// Dependencies are the collaborators the device authenticators need.
type Dependencies struct {
	// Devices resolves API keys and records device activity. Required.
	Devices Fleet
	// Schools blocks devices of deleted schools. Optional: a nil directory
	// skips the check, which only test graphs use.
	Schools SchoolDirectory
	// StaffPIN verifies personal staff credentials; build it with StaffPIN.
	// Optional: a nil verifier rejects requests that present a credential.
	StaffPIN device.StaffPINAuthenticator
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
	var schools device.SchoolLookup
	if deps.Schools != nil {
		schools = schoolLookup{schools: deps.Schools}
	}
	return &Authenticators{authenticator: device.NewAuthenticator(device.Dependencies{
		Devices:     fleetDirectory{fleet: deps.Devices},
		BindTenant:  bindTenant,
		Schools:     schools,
		StaffPIN:    deps.StaffPIN,
		PIN:         pinResolver(deps.Settings),
		FallbackPIN: deps.FallbackPIN,
	})}
}

// Device authenticates API key plus device PIN and binds verified staff.
func (a *Authenticators) Device() Middleware { return a.authenticator.Device() }

// DeviceOnly authenticates the API key alone.
func (a *Authenticators) DeviceOnly() Middleware { return a.authenticator.DeviceOnly() }

// StaffPIN adapts the retained staff PIN verification to the device port.
// The verified row's type stays opaque to this owner: only its identity and
// tenant are copied into the principal handlers see. The type parameters are
// what keep Device Fleet from importing the people-directory staff model; a
// plain adapter over that row would be a compatibility import the target
// policy does not grant.
func StaffPIN[Staff interface {
	*Row
	GetID() any
	GetTenantID() int64
}, Row any](verify func(ctx context.Context, tenantID, staffID int64, pin string) (Staff, error)) device.StaffPINAuthenticator {
	if verify == nil {
		return nil
	}
	return staffPINAuthenticator[Staff, Row]{verify: verify}
}

type staffPINAuthenticator[Staff interface {
	*Row
	GetID() any
	GetTenantID() int64
}, Row any] struct {
	verify func(ctx context.Context, tenantID, staffID int64, pin string) (Staff, error)
}

func (a staffPINAuthenticator[Staff, Row]) AuthenticateStaffPIN(ctx context.Context, tenantID, staffID int64, pin string) (*device.AuthenticatedStaff, error) {
	staff, err := a.verify(ctx, tenantID, staffID, pin)
	if err != nil {
		return nil, err
	}
	if staff == nil {
		return nil, errors.New("staff PIN verification returned no staff")
	}
	id, ok := staff.GetID().(int64)
	if !ok || id <= 0 {
		return nil, fmt.Errorf("staff PIN verification returned an unusable staff id %v", staff.GetID())
	}
	return &device.AuthenticatedStaff{ID: id, TenantID: staff.GetTenantID()}, nil
}

// fleetDirectory serves the device port from the public capability.
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
	return d.fleet.UpdateDeviceLastSeen(ctx, deviceID, seenAt)
}

// schoolLookup classifies the school read for the adapter. It runs before
// the tenant transaction middleware, so the repository falls back to the raw
// connection; platform.schools is not behind RLS.
type schoolLookup struct{ schools SchoolDirectory }

func (l schoolLookup) IsSchoolDeleted(ctx context.Context, schoolID int64) (bool, error) {
	school, err := l.schools.GetSchoolByID(ctx, schoolID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, fmt.Errorf("%w: %w", device.ErrSchoolNotFound, err)
		}
		if isTransientDBErr(err) {
			return false, fmt.Errorf("%w: %w", device.ErrSchoolLookupUnavailable, err)
		}
		return false, err
	}
	if school == nil {
		return false, device.ErrSchoolNotFound
	}
	return school.IsDeleted(), nil
}

// isTransientDBErr returns true for errors that indicate a temporary
// connectivity problem (net timeouts, connection resets, context
// cancellation, PostgreSQL connection-class SQLSTATE 08xxx). Everything else
// (bad queries, permission errors, serialization failures) returns false so
// the adapter fails closed. Wrapped repository errors are unwrapped through
// the standard error chain.
func isTransientDBErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var pgErr pgdriver.Error
	if errors.As(err, &pgErr) {
		code := pgErr.Field('C')
		return len(code) >= 2 && code[:2] == "08"
	}
	return false
}

// bindTenant makes the device's school the ambient tenant. Device routes do
// not use jwt.TenantMiddleware, so an unusable tenant id is reported here.
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
		pin, err := settings.ResolveStringForTenant(ctx, tenantID, configModel.KeyOGSDevicePIN)
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
