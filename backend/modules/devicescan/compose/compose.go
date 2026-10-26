// Package compose builds the device-scan workflow (#2698) over the public
// Device Fleet, Student Presence and Facilities capabilities and binds its
// consumer-owned ports to the retained people, presence, activity, education,
// pickup and settings services, the device principals and the tenant runtime.
// Every retained binding here is a compatibility permission, not a target
// dependency: it goes when the owner behind it exposes the seam publicly.
package compose

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/ports"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// DeviceScan is the composed kiosk scan capability the roots hand to the
// HTTP resources, including the open-room destination booking (#3067).
type DeviceScan interface {
	devicescan.DeviceScan
	devicescan.OpenRoomBooking
}

// Fleet is the part of the Device Fleet capability the scans use.
type Fleet = application.Fleet

// Presence is the part of the Student Presence capability the scans read.
type Presence interface {
	application.Presence
	roomSessionQuery
}

// Rooms is the part of the Facilities capability the scans use.
type Rooms = application.Rooms

// Rosters resolves the day rosters of running blocks. The roots bind it to
// Timetable, which maps the Student Presence sessions to its day rosters.
type Rosters = ports.Rosters

// PickupReader is the slice of the retained pickup schedule service the
// scans read.
type PickupReader interface {
	GetEffectivePickupTimeForDate(ctx context.Context, studentID int64, date calendar.Date) (*careplan.EffectivePickupTime, error)
}

// Dependencies are the collaborators of the device-scan workflow. Fleet,
// Presence, Rooms, Active, Users and Settings are required; the others may be nil in
// narrow graphs and disable the flows that need them.
type Dependencies struct {
	Principal devicescan.DevicePrincipal
	Fleet     Fleet
	Presence  Presence
	Rooms     Rooms
	// Active is the retained presence service that still owns the visit,
	// session and attendance transitions.
	Active studentpresence.Presence
	// Users is the retained people service the cards resolve through.
	Users PersonDirectory
	// Activities is the retained activity catalog the special rooms
	// provision from.
	Activities application.ActivityCatalog
	// Rosters reads the day rosters of running blocks for the Schulhof
	// scan rule (#3282).
	Rosters Rosters
	// Education resolves the child's group for the daily-checkout gate.
	Education SchoolGroups
	// Pickups reads the effective pickup plan.
	Pickups PickupReader
	// OpenRooms books independent stays in released rooms (#3067). The root
	// binds it to the open-room move workflow; nil refuses the booking.
	OpenRooms devicescan.OpenRoomMover
	// Settings resolves tenant settings and is required.
	Settings ScanSettings
	// Now is the workflow clock; nil means the wall clock.
	Now    func() time.Time
	Logger *slog.Logger
}

// New composes the device-scan workflow.
func New(deps Dependencies, bindings Bindings) DeviceScan {
	if deps.Fleet == nil || deps.Presence == nil || deps.Rooms == nil || deps.Active == nil || deps.Users == nil {
		panic("device scan composition: fleet, presence, rooms, active and users are required")
	}
	if deps.Principal == nil {
		panic("device scan composition: principal is required")
	}
	if deps.Settings == nil {
		panic("device scan composition: settings are required")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	clock := bindings.Clock
	var activities application.ActivityCatalog
	if deps.Activities != nil {
		activities = deps.Activities
	}
	return application.NewService(application.Dependencies{
		Fleet:      deps.Fleet,
		Presence:   deps.Presence,
		Rooms:      deps.Rooms,
		Principals: deps.Principal,
		People:     people{PersonDirectory: deps.Users},
		Visits:     bindings.Visits,
		Sessions:   bindings.Sessions,
		Attendance: bindings.Attendance,
		Activities: activities,
		Rosters:    deps.Rosters,
		Groups:     bindings.Groups,
		Pickups:    bindings.Pickups,
		OpenRooms:  deps.OpenRooms,
		Settings:   settings{settings: deps.Settings, presenceModes: deps.Active},
		UnitOfWork: unitOfWork{},
		Clock:      clock,
		Logger:     logger,
	})
}

// clock is the wall clock and the Berlin calendar day the kiosk reasons with.
type clock struct{ now func() time.Time }

func (c clock) Now() time.Time                      { return c.now() }
func (c clock) Day(instant time.Time) calendar.Date { return calendar.DateFromTime(instant) }

// unitOfWork binds the request transaction of the tenant runtime.
type unitOfWork struct{}

func (unitOfWork) RequireTransaction(ctx context.Context) error {
	return application.RequireTransaction(ctx, tenant.TransactionFromContext, tenant.TenantFromContext)
}

func (unitOfWork) MarkRollback(ctx context.Context) { tenant.MarkRollback(ctx) }

func (unitOfWork) BestEffort(ctx context.Context, fn func(context.Context) error) error {
	err := tenant.WithSavepoint(ctx, fn)
	if errors.Is(err, tenant.ErrSavepointControl) {
		return err
	}
	return nil
}

type Bindings struct {
	Visits     ports.Visits
	Sessions   application.SessionOperations
	Attendance ports.Attendance
	Groups     ports.Groups
	Pickups    ports.Pickups
	Clock      ports.Clock
}
type roomSessionQuery interface {
	QueryLiveGroups(context.Context, studentpresence.LiveGroupFilter) ([]studentpresence.LiveGroup, error)
	QueryGroupSupervisions(context.Context, studentpresence.GroupSupervisionFilter) ([]studentpresence.GroupSupervision, error)
}
