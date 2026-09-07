// Package compose builds the Device Fleet owner from its adapters. It is the
// only place that knows about Bun, the shared tenant runtime, and the
// cross-owner sources the info-point dashboard reads.
package compose

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/randstr"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/ports"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
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

// PresenceQuery is the subset of the Student Presence capability the
// dashboard reads.
type PresenceQuery interface {
	ListVisits(context.Context, studentpresence.VisitFilter) ([]studentpresence.Visit, error)
	ListAttendance(context.Context, studentpresence.AttendanceFilter) ([]studentpresence.Attendance, error)
}

// Dependencies are the collaborators the Device Fleet owner needs.
type Dependencies struct {
	DB        *bun.DB
	Rooms     facilitiesModule.Query
	Presence  PresenceQuery
	Dashboard DashboardSources
	Tenants   TenantFacts
	Now       func() time.Time
	// OnlineWindow resolves the per-tenant iot.device_online_window_minutes
	// setting. Graphs without a settings service leave it unset and the owner
	// falls back to defaultDeviceOnlineWindow.
	OnlineWindow func(context.Context) time.Duration
	Observe      func(Observation)
}

// New composes the Device Fleet module.
//
// Device authentication and the public dashboard route both resolve a row
// before any tenant is known, so the store applies the caller's tenant as a
// predicate only when there is one; RLS remains the boundary otherwise.
func New(dependencies Dependencies) (*devicefleet.Module, error) {
	if dependencies.DB == nil || dependencies.Rooms == nil || dependencies.Observe == nil {
		return nil, errors.New("devicefleet compose: database, rooms, and observer are required")
	}
	now := dependencies.Now
	if now == nil {
		now = time.Now
	}
	// Device-only graphs (the repository factory, CLI roots) never serve the
	// public dashboard. They leave its collaborators unset and every
	// dashboard read then fails loudly instead of returning a partial screen.
	presence := ports.PresenceReader(presenceReader{presence: dependencies.Presence})
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
		observation.Err = mapError(observation.Err)
		dependencies.Observe(observation)
	}
	service := application.New(application.Dependencies{
		Devices:   postgres.NewDeviceStore(runtime),
		Displays:  postgres.NewDisplayStore(runtime),
		Rooms:     roomDirectory{rooms: dependencies.Rooms},
		Presence:  presence,
		Dashboard: dashboard,
		Tenants:   tenants,
		Tx:        transaction{db: dependencies.DB},
		Tokens:    displayTokens{},
		APIKeys:   deviceAPIKeys{},
		Now:       ports.Clock(now),
		Window:    ports.WindowResolver(dependencies.OnlineWindow),
		Observe:   observe,
	})
	return devicefleet.NewModule(engine{service: service, observe: observe, now: now}), nil
}

// databaseRuntime resolves the ambient transaction and tenant. A missing
// tenant is not an error: device authentication and the public dashboard
// route both run before one exists.
func databaseRuntime(db *bun.DB) postgres.Database {
	return func(ctx context.Context) (bun.IDB, int64, error) {
		tenantID := tenant.FromContext(ctx)
		transaction, hasTransaction := tenant.TransactionFromContext(ctx)
		if !hasTransaction {
			return db, tenantID, nil
		}
		switch tx := transaction.(type) {
		case bun.Tx:
			return tx, tenantID, nil
		case *bun.Tx:
			if tx != nil {
				return tx, tenantID, nil
			}
			return db, tenantID, nil
		default:
			return nil, 0, fmt.Errorf("devicefleet postgres: unsupported transaction %T", transaction)
		}
	}
}

type transaction struct{ db *bun.DB }

func (t transaction) RunWrite(ctx context.Context, callback func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return callback(ctx)
	}
	if tenant.FromContext(ctx) <= 0 {
		// Operator and CLI graphs write without an ambient tenant exactly as
		// the retired repositories did; RLS still bounds the statement.
		return callback(ctx)
	}
	return tenant.WithinCurrentTenant(ctx, callback)
}

func (t transaction) AdminScope(ctx context.Context, callback func(context.Context) error) error {
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

type presenceReader struct{ presence PresenceQuery }

func (r presenceReader) ListOpenVisits(ctx context.Context) ([]domain.PresentVisit, error) {
	visits, err := r.presence.ListVisits(ctx, studentpresence.VisitFilter{OpenOnly: true})
	if err != nil {
		return nil, err
	}
	result := make([]domain.PresentVisit, 0, len(visits))
	for _, visit := range visits {
		if visit.ExitTime != nil {
			continue
		}
		result = append(result, domain.PresentVisit{ActiveGroupID: visit.ActiveGroupID, StudentID: visit.StudentID})
	}
	return result, nil
}

func (r presenceReader) ListPresentStudents(ctx context.Context, today time.Time) ([]int64, error) {
	day := today.Format(time.DateOnly)
	attendance, err := r.presence.ListAttendance(ctx, studentpresence.AttendanceFilter{FromDate: day, UntilDate: day})
	if err != nil {
		return nil, err
	}
	present := make([]int64, 0, len(attendance))
	for _, entry := range attendance {
		if entry.CheckOutTime == nil {
			present = append(present, entry.StudentID)
		}
	}
	return present, nil
}

// displayTokens mints 32 random bytes as URL-safe base64 and persists only
// the SHA-256 hex digest, so a database leak exposes no usable dashboard URL.
type displayTokens struct{}

func (displayTokens) Mint() (string, string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", "", fmt.Errorf("failed to generate display token: %w", err)
	}
	raw := base64.RawURLEncoding.EncodeToString(buffer)
	return raw, displayTokens{}.Hash(raw), nil
}

func (displayTokens) Hash(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:])
}

type deviceAPIKeys struct{}

func (deviceAPIKeys) Mint() (string, error) { return randstr.APIKey() }
