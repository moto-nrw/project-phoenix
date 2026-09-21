package services

import (
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce/legacy/timetracking"
	"github.com/moto-nrw/project-phoenix/services/iot/staffclock"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type WorkSessionTestModule struct {
	WorkSession timetracking.WorkSessionService
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
	service := timetracking.NewWorkSessionService(r.WorkSession, r.WorkSessionBreak, NewWorkSessionAudit(r.WorkSessionEdit),
		r.StaffAbsence, r.GroupSupervisor, r.ActiveGroup, WorkSessionStaff(r.Staff), NewWorkSessionSchedules(r.StaffWorkSchedule), NewWorkSessionTimeModels(r.WorkTimeModel), PresenceSettings(settings.Settings), slog.Default(), db, RenderTimeTrackingPDF, RenderTimeTrackingWorkbook,
		timetracking.WithWorkSessionShifts(NewTimeTrackingShifts(r.StaffShift)), timetracking.WithWorkSessionEvents(TimeTrackingEvents(deliveryCompose.NewRealtimeHub(slog.Default()))))
	return WorkSessionTestModule{WorkSession: service, StaffClock: newStaffClockService(identity.Users, rfid.RFID, service)}, nil
}
