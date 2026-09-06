package services

import (
	"log/slog"

	iotcheckin "github.com/moto-nrw/project-phoenix/services/iot/checkin"
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

type CheckinTestModule struct{ Checkin *iotcheckin.CheckinService }

func NewCheckinTestModule(db *bun.DB, unit tenant.UnitOfWork) (CheckinTestModule, error) {
	module, err := NewActiveTestModule(db, unit)
	if err != nil {
		return CheckinTestModule{}, err
	}
	return CheckinTestModule{Checkin: iotcheckin.NewCheckinService(iotcheckin.CheckinServiceDeps{
		Active: module.Active, Users: module.Users, Facilities: module.Facilities,
		Activities: module.Activities, Settings: module.Settings, Pickup: module.PickupSchedule,
		Education: module.Education, Logger: slog.Default(), DailyCheckoutFallback: currentFactoryConfig().StudentDailyCheckoutTime,
	})}, nil
}
