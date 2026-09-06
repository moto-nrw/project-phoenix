package services

import (
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
	"github.com/moto-nrw/project-phoenix/tenant"
	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	reminderCompose "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/compose"
	reminderPorts "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/ports"
	"github.com/uptrace/bun"
)

type RemindersTestModule struct {
	Reminders   reminder.Query
	Settings    config.SettingsService
	UserContext usercontext.UserContextService
}

func NewRemindersTestModule(db *bun.DB, unit tenant.UnitOfWork, clocks ...func() time.Time) (RemindersTestModule, error) {
	rooms, err := repositories.NewFacilities(db)
	if err != nil {
		return RemindersTestModule{}, err
	}
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
	service := reminderCompose.NewQuery(reminderPorts.QueryDependencies{
		Clock:        reminderClock(clocks...),
		CurrentStaff: reminderStaffIdentity(groups.UserContext),
		Settings:     reminderSettings{settings.Settings}, Attendance: reminderAttendanceReader{source: repositories.NewAttendanceTestRepository(db)}, Pickup: reminderPickupReader{source: pickup},
		Instance: reminderTimetableReader{source: r.Timetable}, Room: reminderRoomReader{source: rooms},
		Student: reminderStudentReader{source: r.Student}, Person: reminderPersonReader{source: r.Person}, Supervision: reminderSupervisionReader{source: groups.Active},
		Visits: r.ActiveVisit, Logger: slog.Default(), BulkSupervision: reminderBulkSupervisionReader{source: r.GroupSupervisor}, BulkInstanceStaff: reminderTimetableReader{source: r.Timetable},
	})
	return RemindersTestModule{Reminders: service, Settings: settings.Settings, UserContext: groups.UserContext}, nil
}
