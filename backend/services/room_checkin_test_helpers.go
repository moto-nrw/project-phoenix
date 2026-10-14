package services

import (
	"log/slog"
	"time"

	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type RoomsTestModule struct {
	ActiveTestModule
	ListExport *lists.RendererService
}

func NewRoomsTestModule(db *bun.DB, unit tenant.UnitOfWork) (RoomsTestModule, error) {
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

func NewCheckinTestModule(db *bun.DB, unit tenant.UnitOfWork, clocks ...func() time.Time) (CheckinTestModule, error) {
	module, err := NewActiveTestModule(db, unit, clocks...)
	if err != nil {
		return CheckinTestModule{}, err
	}
	rooms, err := facilitiesCompose.NewUnobserved(db)
	if err != nil {
		return CheckinTestModule{}, err
	}
	logger := slog.Default()
	rosters := SessionRosters{Sessions: NewLoggedStudentPresence(db, logger), Roster: NewUnobservedTimetableDependencies(db).Capability}
	scan := devicescanCompose.New(devicescanCompose.Dependencies{
		Fleet:      module.IoT.Fleet(),
		Presence:   NewLoggedStudentPresence(db, logger),
		Rooms:      rooms,
		Active:     module.Active,
		Users:      module.Users,
		Activities: module.Activities,
		Rosters:    rosters,
		Education:  module.Education,
		Pickups:    module.PickupSchedule,
		Settings:   module.Settings,
		Now:        OptionalClock(clocks),
		Logger:     logger,
	})
	return CheckinTestModule{ActiveTestModule: module, DeviceScan: scan, Rosters: rosters}, nil
}
