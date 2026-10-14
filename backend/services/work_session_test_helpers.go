package services

import (
	"log/slog"
	"time"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	staffclock "github.com/moto-nrw/project-phoenix/workflows/staffclock/compose"
	"github.com/uptrace/bun"
)

// WorkSessionTestModule is the stamp side of the Workforce time-tracking
// composition with the staff-clock workflow over it.
type WorkSessionTestModule struct {
	TimeTracking *workforceCompose.TimeTracking
	StaffClock   *staffclock.Service
}

func NewWorkSessionTestModule(db *bun.DB, unit tenant.UnitOfWork, clocks ...func() time.Time) (WorkSessionTestModule, error) {
	r, err := NewWorkSessionTestRepositories(db, clocks...)
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
	rfid, err := NewRFIDTestRepositories(db)
	if err != nil {
		return WorkSessionTestModule{}, err
	}
	audits := NewTestAuditPersistence(db, nil)
	timeTracking, err := newTimeTrackingTestModule(db, r, timeTrackingTestGraph{
		Audits: TimeTrackingAudits{
			WorkSessionEdit: r.WorkSessionEdit, TimeTrackingDeletion: audits.TimeTrackingDeletion,
			DataAccessLog: audits.DataAccessLog, TimeTrackingAuditLog: audits.TimeTrackingAuditLog,
		},
		Settings: settings.Settings, People: identity.Users, Holidays: noHolidays{},
		Events: TimeTrackingEvents(deliveryCompose.NewRealtimeHub(slog.Default())), Logger: slog.Default(),
	})
	if err != nil {
		return WorkSessionTestModule{}, err
	}
	return WorkSessionTestModule{TimeTracking: timeTracking, StaffClock: NewStaffClockService(identity.Users, rfid.RFID, timeTracking.TimeClock)}, nil
}
