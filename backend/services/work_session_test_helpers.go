package services

import (
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/iot/staffclock"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type WorkSessionTestModule struct {
	WorkSession active.WorkSessionService
	StaffClock  *staffclock.Service
}

func NewWorkSessionTestModule(db *bun.DB, unit tenant.UnitOfWork, clocks ...func() time.Time) (WorkSessionTestModule, error) {
	r, err := repositories.NewWorkSessionTestRepositories(db, clocks...)
	if err != nil {
		return WorkSessionTestModule{}, err
	}
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return WorkSessionTestModule{}, err
	}
	identity, err := NewRFIDTestModule(db)
	if err != nil {
		return WorkSessionTestModule{}, err
	}
	rfid, err := repositories.NewRFIDTestRepositories(db)
	if err != nil {
		return WorkSessionTestModule{}, err
	}
	membership, err := repositories.NewSchoolMembership(db)
	if err != nil {
		return WorkSessionTestModule{}, err
	}
	r = r.WithConfigRuntime(newSettingsRuntime(db, &unit).WithSchoolMembership(membership))
	service := active.NewWorkSessionService(r.WorkSession, r.WorkSessionBreak, r.WorkSessionEdit,
		r.StaffAbsence, r.GroupSupervisor, r.ActiveGroup, r.Staff, r.StaffWorkSchedule, r.WorkTimeModel, settings.Settings, slog.Default(), db)
	service.SetStaffShiftRepo(r.StaffShift)
	service.(interface{ SetBroadcaster(realtime.Broadcaster) }).SetBroadcaster(deliveryCompose.NewRealtimeHub(slog.Default()))
	return WorkSessionTestModule{WorkSession: service, StaffClock: staffclock.NewService(identity.Users, newRFIDCardLookup(rfid.RFID.FindByID), service)}, nil
}
