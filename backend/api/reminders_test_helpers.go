package api

import (
	"context"
	"log/slog"
	"time"

	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/compose/vocabulary"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/tenant"
	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	reminderCompose "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/compose"
	reminderPorts "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/ports"
)

type RemindersTestModule struct {
	Reminders   reminder.Query
	Settings    settingsCompose.SettingsService
	UserContext *CallerRows
}

func NewRemindersTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, clocks ...func() time.Time) (RemindersTestModule, error) {
	rooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		return RemindersTestModule{}, err
	}
	r, err := NewTimetableTestRepositories(db)
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
	students, err := NewPeopleDirectory(db)
	if err != nil {
		return RemindersTestModule{}, err
	}
	carePlan, err := NewCarePlan(db, students, r.InstanceStudent)
	if err != nil {
		return RemindersTestModule{}, err
	}
	baseline, err := careplanCompose.NewPickupBaselines(carePlan, approvedOfferings, func(ctx context.Context) (bool, error) {
		return settings.Settings.ResolveBool(ctx, configModels.KeyEnrollmentBookingsAuthoritative)
	})
	if err != nil {
		return RemindersTestModule{}, err
	}
	autoExcusal, err := careplanCompose.NewPickupAutoExcusal(careplanCompose.PickupExcusalDependencies{
		DB: db.Pool(), Records: carePlan, Baselines: baseline, Blocks: services.NewLoggedStudentPresence(db.Pool(), slog.Default()), Preview: services.NewPickupExcusalTimetable(r.Timetable, services.NewPartialAbsencePreview(db.Pool())),
	})
	if err != nil {
		return RemindersTestModule{}, err
	}
	pickupStudents, err := services.PickupPlanStudents(students)
	if err != nil {
		return RemindersTestModule{}, err
	}
	pickup, err := careplanCompose.NewPickupSchedules(db.Pool(), carePlan, baseline, autoExcusal, pickupStudents, slog.Default())
	if err != nil {
		return RemindersTestModule{}, err
	}
	service := reminderCompose.NewQuery(reminderPorts.QueryDependencies{
		Clock:        services.ReminderClock(clocks...),
		CurrentStaff: services.ReminderStaffIdentity(groups.UserContext),
		Settings:     services.ReminderSettings{SettingsService: settings.Settings}, Attendance: services.NewLoggedStudentPresence(db.Pool(), slog.Default()), Pickup: services.ReminderPickupReader{Source: pickup},
		Instance: services.ReminderTimetableReader{Source: r.Timetable}, Room: services.ReminderRoomReader{Source: rooms},
		Student: services.ReminderStudentReader{Source: r.Student}, Person: services.ReminderPersonReader{Source: r.Person}, Supervision: services.ReminderSupervisionReader{Source: groups.Active, Presence: services.NewLoggedStudentPresence(db.Pool(), slog.Default())},
		Visits: services.ReminderVisitReader{Source: services.NewLoggedStudentPresence(db.Pool(), slog.Default())}, Logger: slog.Default(), BulkSupervision: services.ReminderBulkSupervisionReader{Source: r.GroupSupervisor}, BulkInstanceStaff: services.ReminderTimetableReader{Source: r.Timetable},
	})
	return RemindersTestModule{Reminders: service, Settings: settings.Settings, UserContext: groups.UserContext}, nil
}
