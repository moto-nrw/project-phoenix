//go:build testhooks

package api

import (
	"log/slog"
	"time"

	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type RoomsTestModule struct {
	ActiveTestModule
	ListExport *lists.RendererService
}

func NewRoomsTestModule(db transactionruntime.Database, unit tenant.UnitOfWork) (RoomsTestModule, error) {
	active, err := NewActiveTestModule(db, unit)
	if err != nil {
		return RoomsTestModule{}, err
	}
	return RoomsTestModule{ActiveTestModule: active, ListExport: lists.NewRendererService()}, nil
}

// CheckinTestModule composes the device-scan workflow (#2698) over the
// active test graph, the way the production root does. Daily-checkout gates
// depend on tenant settings alone, never on the developer's environment.
// Rosters is the block roster lookup the workflow reads.
type CheckinTestModule struct {
	ActiveTestModule
	DeviceScan devicescanCompose.DeviceScan
	Rosters    devicescanCompose.Rosters
}

func NewCheckinTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, clocks ...func() time.Time) (CheckinTestModule, error) {
	module, err := NewActiveTestModule(db, unit, clocks...)
	if err != nil {
		return CheckinTestModule{}, err
	}
	rooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		return CheckinTestModule{}, err
	}
	logger := slog.Default()
	rosters := NewUnobservedTimetableDependencies(db).Capability
	scan := devicescanCompose.New(devicescanCompose.Dependencies{
		Fleet:      module.IoT.Fleet(),
		Presence:   NewLoggedStudentPresence(db, logger),
		Rooms:      rooms,
		Active:     module.Active,
		Users:      NewDeviceScanPeople(module.Users),
		Activities: DeviceActivityCatalog(module.Activities),
		Rosters:    rosters,
		Education:  module.Education,
		Pickups:    module.PickupSchedule,
		Settings:   newDeviceScanSettings(module.Settings),
		Now:        OptionalClock(clocks),
		Logger:     logger,
	})
	return CheckinTestModule{ActiveTestModule: module, DeviceScan: scan, Rosters: rosters}, nil
}

// NewCheckinTestPresence builds the same unobserved presence reader used by
// the kiosk contract suite, without exposing its composition dependency.
func NewCheckinTestPresence(db transactionruntime.Database) (*studentpresence.Module, error) {
	return presenceCompose.New(presenceCompose.Dependencies{
		DB: db.Pool(), Observe: func(presenceCompose.Observation) {},
	})
}
