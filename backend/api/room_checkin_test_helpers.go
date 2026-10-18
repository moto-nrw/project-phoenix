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

// NewRoomsTestModule composes the active graph the room routes run over and
// returns it with the list export the room lists render through.
func NewRoomsTestModule(db transactionruntime.Database, unit tenant.UnitOfWork) (ActiveTestModule, *lists.RendererService, error) {
	active, err := NewActiveTestModule(db, unit)
	if err != nil {
		return ActiveTestModule{}, nil, err
	}
	return active, lists.NewRendererService(), nil
}

// CheckinTestModule composes the device-scan workflow (#2698) over the
// active test graph, the way the production root does. Daily-checkout gates
// depend on tenant settings alone, never on the developer's environment.
// Rosters is the block roster lookup the workflow reads.
type CheckinTestModule struct {
	DeviceScan devicescanCompose.DeviceScan
	Rosters    devicescanCompose.Rosters
}

func NewCheckinTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, clocks ...func() time.Time) (CheckinTestModule, error) {
	module, err := NewActiveTestModule(db, unit, clocks...)
	if err != nil {
		return CheckinTestModule{}, err
	}
	moduleDevices := NewActiveTestDevices(db, unit, clocks...)
	modulePresence := NewActiveTestPresence(db, unit, clocks...)
	modulePeople := NewActiveTestPeople(db, unit, clocks...)
	moduleTimetable := NewActiveTestTimetable(db, unit, clocks...)
	moduleEducation := NewActiveTestEducation(db, unit, clocks...)
	moduleSettings := NewActiveTestSettings(db, unit, clocks...)
	rooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		return CheckinTestModule{}, err
	}
	logger := slog.Default()
	rosters := MustNewUnobservedTimetable(db)
	scan := devicescanCompose.New(devicescanCompose.Dependencies{Principal: DeviceScanPrincipal,
		Fleet:      moduleDevices.IoT.Fleet(),
		Presence:   NewLoggedStudentPresence(db, logger),
		Rooms:      rooms,
		Active:     modulePresence.Active,
		Users:      NewDeviceScanPeople(modulePeople.Users),
		Activities: DeviceActivityCatalog(moduleTimetable.Activities),
		Rosters:    rosters,
		Education:  moduleEducation.Education,
		Pickups:    module.PickupSchedule,
		Settings:   newDeviceScanSettings(moduleSettings.Settings),
		Now:        OptionalClock(clocks),
		Logger:     logger,
	})
	return CheckinTestModule{DeviceScan: scan, Rosters: rosters}, nil
}

// NewCheckinTestPresence builds the same unobserved presence reader used by
// the kiosk contract suite, without exposing its composition dependency.
func NewCheckinTestPresence(db transactionruntime.Database) (*studentpresence.Module, error) {
	return presenceCompose.New(presenceCompose.Dependencies{
		DB: db.Pool(), Observe: func(presenceCompose.Observation) {},
	})
}
