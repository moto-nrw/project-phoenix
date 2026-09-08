package services

import (
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type RoomsTestModule struct {
	ActiveTestModule
	ListExport *listexport.RendererService
}

func NewRoomsTestModule(db *bun.DB, unit tenant.UnitOfWork) (RoomsTestModule, error) {
	active, err := NewActiveTestModule(db, unit)
	if err != nil {
		return RoomsTestModule{}, err
	}
	return RoomsTestModule{ActiveTestModule: active, ListExport: listexport.NewService()}, nil
}

// CheckinTestModule composes the device-scan workflow (#2698) over the
// active test graph, the way the production root does. The process
// checkout-time fallback stays empty so the daily-checkout gates depend on
// tenant settings alone, never on the developer's environment.
type CheckinTestModule struct {
	ActiveTestModule
	DeviceScan devicescanCompose.DeviceScan
}

func NewCheckinTestModule(db *bun.DB, unit tenant.UnitOfWork, clocks ...func() time.Time) (CheckinTestModule, error) {
	module, err := NewActiveTestModule(db, unit, clocks...)
	if err != nil {
		return CheckinTestModule{}, err
	}
	rooms, err := repositories.NewFacilities(db)
	if err != nil {
		return CheckinTestModule{}, err
	}
	logger := slog.Default()
	scan := devicescanCompose.New(devicescanCompose.Dependencies{
		Fleet:      module.IoT.Fleet(),
		Presence:   newStudentPresence(db, logger),
		Rooms:      rooms,
		Active:     module.Active,
		Users:      module.Users,
		Activities: module.Activities,
		Education:  module.Education,
		Pickups:    module.PickupSchedule,
		Settings:   module.Settings,
		Now:        optionalClock(clocks),
		Logger:     logger,
	})
	return CheckinTestModule{ActiveTestModule: module, DeviceScan: scan}, nil
}
