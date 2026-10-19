// Package compose builds the Device Fleet owner from its adapters. It is the
// only place that knows about Bun, the shared tenant runtime, and the
// cross-owner sources the info-point dashboard reads.
package compose

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/adapters/credentials"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/ports"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Observation is one recorded operation outcome.
type Observation = ports.Observation

// ErrDisplayNotFound is what a TenantFacts implementation returns when the
// tenant behind a display token no longer exists. The owner turns it into
// devicefleet.ErrDisplayNotFound, so a dead link never reveals tenant state.
var ErrDisplayNotFound = domain.ErrDisplayNotFound

// School is the tenant fact the dashboard reads before a tenant is scoped.
type School = domain.School

// ActiveSession is one running room session the dashboard renders.
type ActiveSession = domain.ActiveSession

// ActivityTemplate is one activity definition with its category.
type ActivityTemplate = domain.ActivityTemplate

// PlannedActivity is one of today's activity instances.
type PlannedActivity = domain.PlannedActivity

// PickupTime is one present student's effective pickup wall clock today.
type PickupTime = domain.PickupTime

// DashboardSources supplies the cross-owner facts the dashboard renders and
// cannot read through a public capability.
type DashboardSources = ports.DashboardSources

// TenantFacts answers the pre-tenant questions the dashboard asks while it
// still holds only a display token. A missing school must be reported as
// devicefleet.ErrDisplayNotFound so a dead link never leaks tenant state.
type TenantFacts = ports.TenantFacts

// Dependencies are the collaborators the Device Fleet owner needs.
type Dependencies struct {
	DB        *bun.DB
	Rooms     facilitiesModule.Query
	Presence  application.PresenceQuery
	Dashboard DashboardSources
	Tenants   TenantFacts
	// Now is the owner's clock. The info-point aggregate derives the rendered
	// calendar day and the "later today" cut-off from it, so a graph that
	// serves the dashboard must supply Berlin time (timezone.Now); device-only
	// graphs may leave it unset and get plain instants.
	Now func() time.Time
	// OnlineWindowMinutes resolves the raw iot.device_online_window_minutes
	// value. Graphs without a settings service leave it unset; unset and
	// non-positive minutes fall back to the owner's default window.
	OnlineWindowMinutes func(context.Context) int
	// APIKeys mints device API keys; the root binds Security Runtime's minter.
	APIKeys interface{ Mint() (string, error) }
	Observe func(Observation)
}

// New composes the Device Fleet module.
//
// Device authentication and the public dashboard route both resolve a row
// before any tenant is known, so the store applies the caller's tenant as a
// predicate only when there is one; RLS remains the boundary otherwise.
func New(dependencies Dependencies) (*devicefleet.Module, error) {
	if dependencies.DB == nil || dependencies.Rooms == nil || dependencies.APIKeys == nil || dependencies.Observe == nil {
		return nil, errors.New("devicefleet compose: database, rooms, API keys and observer are required")
	}
	now := dependencies.Now
	if now == nil {
		if dependencies.Dashboard != nil || dependencies.Tenants != nil || dependencies.Presence != nil {
			return nil, errors.New("devicefleet compose: the info-point dashboard requires an explicit Berlin clock")
		}
		now = time.Now
	}
	// Device-only graphs (the repository factory, CLI roots) never serve the
	// public dashboard. They leave its collaborators unset and every
	// dashboard read then fails loudly instead of returning a partial screen.
	presence := ports.PresenceReader(application.PresenceReader{Presence: dependencies.Presence})
	if dependencies.Presence == nil {
		presence = uncomposedDashboard{}
	}
	dashboard := dependencies.Dashboard
	if dashboard == nil {
		dashboard = uncomposedDashboard{}
	}
	tenants := dependencies.Tenants
	if tenants == nil {
		tenants = uncomposedDashboard{}
	}
	runtime := databaseRuntime(dependencies.DB)
	observe := func(observation Observation) {
		observation.Err = application.MapError(observation.Err)
		dependencies.Observe(observation)
	}
	service := application.New(application.Dependencies{
		Devices:   postgres.NewDeviceStore(runtime),
		Displays:  postgres.NewDisplayStore(runtime),
		Scans:     postgres.NewUnregisteredTagScanStore(runtime),
		Rooms:     roomDirectory{rooms: dependencies.Rooms},
		Presence:  presence,
		Dashboard: dashboard,
		Tenants:   tenants,
		Tx:        transaction{db: dependencies.DB},
		Tokens:    credentials.DisplayTokens{},
		APIKeys:   dependencies.APIKeys,
		Now:       ports.Clock(now),
		Window:    devicefleet.OnlineWindowFromMinutes(dependencies.OnlineWindowMinutes),
		Observe:   observe,
	})
	return devicefleet.NewModule(application.NewEngine(service, observe, now)), nil
}

// databaseRuntime resolves the ambient transaction and tenant. A missing
// tenant is not an error: device authentication and the public dashboard
// route both run before one exists.
func databaseRuntime(db *bun.DB) postgres.Database {
	return func(ctx context.Context) (bun.IDB, postgres.TenantScope, error) {
		id, scopeErr := tenant.TenantFromContext(ctx)
		raw, ok := tenant.TransactionFromContext(ctx)
		if scopeErr != nil && !tenant.IsAdminTx(ctx) {
			return nil, postgres.TenantScope{}, errors.Join(devicefleet.ErrTenantRequired, scopeErr)
		}
		return postgres.ResolveDatabase(raw, ok, id.Int64(), scopeErr, tenant.IsAdminTx(ctx))
	}

}

type transaction struct{ db *bun.DB }

func (t transaction) RunWrite(ctx context.Context, callback func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return callback(ctx)
	}
	id, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return errors.Join(devicefleet.ErrTenantRequired, err)
	}
	return tenant.WithinTenant(ctx, id, callback)
}

func (t transaction) AdminScope(ctx context.Context, callback func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return callback(ctx)
	}
	return tenant.WithAdminTx(ctx, t.db, func(adminCtx context.Context, _ bun.Tx) error {
		return callback(adminCtx)
	})
}

func (t transaction) TenantScope(ctx context.Context, tenantID int64, callback func(context.Context) error) error {
	return tenant.WithTenantTx(ctx, t.db, tenantID, func(tenantCtx context.Context, _ bun.Tx) error {
		return callback(tenantCtx)
	})
}

type roomDirectory struct{ rooms facilitiesModule.Query }

func (d roomDirectory) ListRoomsByID(ctx context.Context, ids []int64) ([]domain.Room, error) {
	rooms, err := d.rooms.ListRoomsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	return toDomainRooms(rooms), nil
}

func (d roomDirectory) ListRooms(ctx context.Context) ([]domain.Room, error) {
	rooms, err := d.rooms.ListRooms(ctx, facilitiesModule.RoomFilter{})
	if err != nil {
		return nil, err
	}
	return toDomainRooms(rooms), nil
}

func toDomainRooms(rooms []facilitiesModule.Room) []domain.Room {
	result := make([]domain.Room, 0, len(rooms))
	for _, room := range rooms {
		result = append(result, domain.Room{
			ID: room.ID, TenantID: room.TenantID, Name: room.Name, Capacity: room.Capacity,
		})
	}
	return result
}
