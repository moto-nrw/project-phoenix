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

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	activitiesSvc "github.com/moto-nrw/project-phoenix/services/activities"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	educationSvc "github.com/moto-nrw/project-phoenix/services/education"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// DeviceScan is the composed kiosk scan capability the roots hand to the
// HTTP resources.
type DeviceScan = devicescan.DeviceScan

// Fleet is the part of the Device Fleet capability the scans use.
type Fleet = application.Fleet

// Presence is the part of the Student Presence capability the scans read.
type Presence = application.Presence

// Rooms is the part of the Facilities capability the scans use.
type Rooms = application.Rooms

// PickupReader is the slice of the retained pickup schedule service the
// scans read.
type PickupReader interface {
	GetEffectivePickupTimeForDate(ctx context.Context, studentID int64, date timezone.Date) (*scheduleSvc.EffectivePickupTime, error)
}

// Dependencies are the collaborators of the device-scan workflow. Fleet,
// Presence, Rooms, Active, Users and Settings are required; the others may be nil in
// narrow graphs and disable the flows that need them.
type Dependencies struct {
	Fleet    Fleet
	Presence Presence
	Rooms    Rooms
	// Active is the retained presence service that still owns the visit,
	// session and attendance transitions.
	Active activeSvc.Service
	// Users is the retained people service the cards resolve through.
	Users usersSvc.PersonService
	// Activities is the retained activity catalog the special rooms
	// provision from.
	Activities activitiesSvc.ActivityService
	// Education resolves the child's group for the daily-checkout gate.
	Education educationSvc.Service
	// Pickups reads the effective pickup plan.
	Pickups PickupReader
	// Settings resolves tenant settings and is required.
	Settings configSvc.SettingsService
	// Now is the workflow clock; nil means the wall clock.
	Now    func() time.Time
	Logger *slog.Logger
}

// New composes the device-scan workflow.
func New(deps Dependencies) DeviceScan {
	if deps.Fleet == nil || deps.Presence == nil || deps.Rooms == nil || deps.Active == nil || deps.Users == nil {
		panic("device scan composition: fleet, presence, rooms, active and users are required")
	}
	if deps.Settings == nil {
		panic("device scan composition: settings are required")
	}
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	clock := clock{now: now}
	var activities ports.Activities
	if deps.Activities != nil {
		activities = activityCatalog{activities: deps.Activities}
	}
	var groups ports.Groups
	if deps.Education != nil {
		groups = groupDirectory{education: deps.Education}
	}
	var pickups ports.Pickups
	if deps.Pickups != nil {
		pickups = pickupPlan{pickups: deps.Pickups}
	}
	return application.NewService(application.Dependencies{
		Fleet:      deps.Fleet,
		Presence:   deps.Presence,
		Rooms:      deps.Rooms,
		Principals: principals{},
		People:     people{users: deps.Users},
		Visits:     visits{active: deps.Active, clock: clock},
		Sessions:   sessions{active: deps.Active, clock: clock},
		Attendance: attendance{active: deps.Active},
		Activities: activities,
		Groups:     groups,
		Pickups:    pickups,
		Settings:   settings{settings: deps.Settings, active: deps.Active},
		UnitOfWork: unitOfWork{},
		Clock:      clock,
		Logger:     logger,
	})
}

// clock is the wall clock and the Berlin calendar day the kiosk reasons with.
type clock struct{ now func() time.Time }

func (c clock) Now() time.Time                      { return c.now() }
func (c clock) Day(instant time.Time) timezone.Date { return timezone.DateFromTime(instant) }

// unitOfWork binds the request transaction of the tenant runtime.
type unitOfWork struct{}

func (unitOfWork) RequireTransaction(ctx context.Context) error {
	if _, ok := tenant.TransactionFromContext(ctx); !ok {
		return errors.New("device scan requires a tenant transaction")
	}
	_, err := tenant.TenantFromContext(ctx)
	return err
}

func (unitOfWork) MarkRollback(ctx context.Context) { tenant.MarkRollback(ctx) }

// mappedError keeps the retained service's message on the wire while
// answering errors.Is for the workflow's port sentinel.
type mappedError struct {
	err      error
	sentinel error
}

func (e mappedError) Error() string        { return e.err.Error() }
func (e mappedError) Unwrap() error        { return e.err }
func (e mappedError) Is(target error) bool { return target == e.sentinel }

func mapError(err, sentinel error) error { return mappedError{err: err, sentinel: sentinel} }
