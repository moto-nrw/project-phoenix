//go:build testhooks

package api

import (
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// NewWorkSessionTestModule composes the stamp side of the Workforce
// time-tracking composition and the staff-clock workflow over it.
func NewWorkSessionTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, clocks ...func() time.Time) (*workforceCompose.TimeTracking, devicescan.StaffClock, error) {
	members := MustNewMembershipTestRepositories(db)
	r, err := NewWorkSessionTestRepositoriesWithMembers(db, members, clocks...)
	if err != nil {
		return nil, nil, err
	}
	timetable, err := NewTimetableTestRepositoriesWithMembers(db, members, clocks...)
	if err != nil {
		return nil, nil, err
	}
	presence, err := NewTimetablePresenceRows(db, timetable, members, clocks...)
	if err != nil {
		return nil, nil, err
	}
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return nil, nil, err
	}
	people, err := NewRFIDTestPeople(db)
	if err != nil {
		return nil, nil, err
	}
	audits := NewTestAuditPersistence(db, nil)
	timeTracking, err := newTimeTrackingTestModule(db, r, members.Staff, presence, settings.Settings, timeTrackingTestGraph{
		Audits: timeTrackingAudits{
			WorkSessionEdit: audits.WorkSessionEdit, TimeTrackingDeletion: audits.TimeTrackingDeletion,
			DataAccessLog: audits.DataAccessLog, TimeTrackingAuditLog: audits.TimeTrackingAuditLog,
		},
		People: people, Holidays: noHolidays{},
		Events: timeTrackingEvents(deliveryCompose.NewRealtimeHub(slog.Default())), ShiftPlanSync: noSickCascade{}, Logger: slog.Default(),
	})
	if err != nil {
		return nil, nil, err
	}
	return timeTracking, newStaffClockService(people, NewPeopleIdentityAccess(db, nil), timeTracking.TimeClock), nil
}
