package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/domain"
)

// DeviceStore is the owner's persistence port over iot.devices.
type DeviceStore interface {
	Create(context.Context, domain.CreateDevice) (domain.Device, domain.OperationStats, error)
	Update(context.Context, domain.UpdateDevice) (domain.Device, domain.OperationStats, error)
	UpdateColumns(context.Context, domain.UpdateDevice, []string) (int64, domain.OperationStats, error)
	UpdateStatusByDeviceID(context.Context, string, domain.DeviceStatus) (int64, domain.OperationStats, error)
	Delete(context.Context, int64) (int64, domain.OperationStats, error)
	FindByID(context.Context, int64, string) (domain.Device, bool, domain.OperationStats, error)
	FindByDeviceID(context.Context, string) (domain.Device, bool, domain.OperationStats, error)
	FindByAPIKey(context.Context, string) (domain.Device, bool, domain.OperationStats, error)
	List(context.Context, domain.DeviceFilter) ([]domain.Device, domain.OperationStats, error)
	ListByIDs(context.Context, []int64) ([]domain.Device, domain.OperationStats, error)
	ListOffline(context.Context, time.Time) ([]domain.Device, domain.OperationStats, error)
	CountByType(context.Context) (map[string]int, domain.OperationStats, error)
	CountByTenant(context.Context) (map[int64]int, domain.OperationStats, error)
	ListByTenants(context.Context, []int64) ([]domain.Device, domain.OperationStats, error)
}

// DisplayStore is the owner's persistence port over display.displays.
type DisplayStore interface {
	Create(context.Context, domain.CreateDisplay) (domain.Display, domain.OperationStats, error)
	Update(context.Context, domain.UpdateDisplay) (int64, domain.OperationStats, error)
	Delete(context.Context, int64) (int64, domain.OperationStats, error)
	FindByID(context.Context, int64) (domain.Display, bool, domain.OperationStats, error)
	List(context.Context) ([]domain.Display, domain.OperationStats, error)
	// FindByTokenHash resolves a display without a tenant predicate. It must
	// only ever run inside the admin transaction supplied by AdminScope: the
	// token is the sole auth signal and no tenant is known yet.
	FindByTokenHash(context.Context, string) (domain.Display, bool, domain.OperationStats, error)
}

// UnregisteredTagScanStore is the owner's persistence port over
// audit.unregistered_tag_scans.
type UnregisteredTagScanStore interface {
	Insert(context.Context, domain.RecordUnregisteredTagScan) (domain.UnregisteredTagScan, domain.OperationStats, error)
	FindByID(context.Context, int64) (domain.UnregisteredTagScan, bool, domain.OperationStats, error)
	List(context.Context, domain.UnregisteredTagScanFilter) ([]domain.UnregisteredTagScan, domain.OperationStats, error)
	// Resolve stamps one still-open scan and reports how many rows matched.
	Resolve(context.Context, domain.ResolveUnregisteredTagScan) (int64, domain.OperationStats, error)
	// DeleteExpired removes the caller tenant's scans older than cutoff through
	// the audit retention function and reports the deleted count.
	DeleteExpired(context.Context, time.Time) (int64, domain.OperationStats, error)
}

// DashboardSources supplies the cross-owner facts the info-point aggregate
// renders and cannot read through a public capability of its own. Every
// method runs inside the display's tenant transaction; the composition root
// maps the owning modules' values into this owner's domain.
//
// today is the calendar day being rendered, already resolved to Berlin by
// the composition root.
type DashboardSources interface {
	ListActiveSessions(ctx context.Context) ([]domain.ActiveSession, error)
	ListActivityTemplates(ctx context.Context) ([]domain.ActivityTemplate, error)
	ListPlannedActivities(ctx context.Context, today time.Time) ([]domain.PlannedActivity, error)
	ListPickupTimes(ctx context.Context, studentIDs []int64, today time.Time) ([]domain.PickupTime, error)
}

// PresenceReader is the Student Presence capability the occupancy and pickup
// panels read. The composition root binds the public module.
type PresenceReader interface {
	ListOpenVisits(ctx context.Context) ([]domain.PresentVisit, error)
	ListPresentStudents(ctx context.Context, today time.Time) ([]int64, error)
}

// RoomDirectory is the Facilities capability device room names and the
// occupancy panel resolve through. No device query ever joins facilities.rooms.
type RoomDirectory interface {
	ListRoomsByID(ctx context.Context, ids []int64) ([]domain.Room, error)
	ListRooms(ctx context.Context) ([]domain.Room, error)
}

// TenantFacts answers the pre-tenant questions the dashboard asks while it
// still holds only a display token.
type TenantFacts interface {
	School(context.Context, int64) (domain.School, error)
	DisplayEnabled(context.Context, int64) (bool, error)
}

// Transaction is the owner's seam onto the shared tenant runtime.
type Transaction interface {
	// RunWrite executes callback inside the caller's ambient tenant
	// transaction, opening one when none exists.
	RunWrite(context.Context, func(context.Context) error) error
	// AdminScope runs callback under the BYPASSRLS role used to resolve a
	// display token to its tenant.
	AdminScope(context.Context, func(context.Context) error) error
	// TenantScope runs callback inside a transaction scoped to tenantID.
	TenantScope(context.Context, int64, func(context.Context) error) error
}

// TokenMinter issues and hashes info-point access tokens.
type TokenMinter interface {
	Mint() (raw string, hash string, err error)
	Hash(raw string) string
}

// APIKeyMinter issues device API keys.
type APIKeyMinter interface {
	Mint() (string, error)
}

// Clock is the owner's time source; the composition root supplies Berlin time.
type Clock func() time.Time

// WindowResolver resolves the per-tenant device-online window. The
// composition root reads the tenant setting; the owner decides what the
// window means.
type WindowResolver func(context.Context) time.Duration

// Observation is one recorded operation outcome.
type Observation struct {
	Operation string
	Duration  time.Duration
	Stats     domain.OperationStats
	Err       error
}

// Observer receives every operation outcome, including rejections.
type Observer func(Observation)
