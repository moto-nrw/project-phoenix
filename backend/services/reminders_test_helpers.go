package services

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/reminders"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type RemindersTestModule struct {
	Reminders   reminders.Service
	Settings    config.SettingsService
	UserContext usercontext.UserContextService
}

func NewRemindersTestModule(db *bun.DB, unit tenant.UnitOfWork) (RemindersTestModule, error) {
	r, err := repositories.NewTimetableTestRepositories(db)
	if err != nil {
		return RemindersTestModule{}, err
	}
	approvedOfferings, err := NewApprovedOfferingTestProjection(db, r.Enrollment())
	if err != nil {
		return RemindersTestModule{}, err
	}
	groups, err := NewGroupsTestModule(db, unit)
	if err != nil {
		return RemindersTestModule{}, err
	}
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return RemindersTestModule{}, err
	}
	baseline := schedule.NewPickupBaselineServiceWithSettings(r.StudentPickupSchedule, approvedOfferings, r.CareOffering, settings.Settings)
	pickup := schedule.NewPickupScheduleServiceWithBulk(r.StudentPickupSchedule, r.StudentPickupException, r.StudentPickupNote,
		r.Student, r.Person, schedule.NewPickupAutoExcusalSyncer(r.StudentPickupException, baseline, r.InstanceStudent, db), baseline, db, slog.Default())
	service := reminders.NewService(reminders.Dependencies{
		Settings: settings.Settings, Attendance: repositories.NewAttendanceTestRepository(db), Pickup: pickup,
		Instance: r.ActivityInstance, Room: r.Room, Student: r.Student, Person: r.Person, Supervision: groups.Active,
		Visits: r.ActiveVisit, Logger: slog.Default(), BulkSupervision: r.GroupSupervisor, BulkInstanceStaff: r.InstanceStaff,
	})
	return RemindersTestModule{Reminders: service, Settings: settings.Settings, UserContext: groups.UserContext}, nil
}
