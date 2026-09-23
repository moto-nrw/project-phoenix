package services

import (
	"context"
	"log/slog"
	"time"

	configModels "github.com/moto-nrw/project-phoenix/models/config"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	reminderCompose "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/compose"
	reminderPorts "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/ports"
	"github.com/uptrace/bun"
)

type RemindersTestModule struct {
	Reminders   reminder.Query
	Settings    config.SettingsService
	UserContext *repositories.CallerRows
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
	students, err := repositories.NewPeopleDirectory(db)
	if err != nil {
		return RemindersTestModule{}, err
	}
	carePlan, err := repositories.NewCarePlan(db, students, r.InstanceStudent)
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
		DB: db, Records: carePlan, Baselines: baseline, Blocks: newStudentPresence(db, slog.Default()), Preview: newPickupExcusalTimetable(r.Timetable, db),
	})
	if err != nil {
		return RemindersTestModule{}, err
	}
	pickup, err := NewPickupSchedules(db, carePlan, students, baseline, autoExcusal, slog.Default())
	if err != nil {
		return RemindersTestModule{}, err
	}
	service := reminderCompose.NewQuery(reminderPorts.QueryDependencies{
		Clock:        reminderClock(clocks...),
		CurrentStaff: reminderStaffIdentity(groups.UserContext),
		Settings:     reminderSettings{settings.Settings}, Attendance: newStudentPresence(db, slog.Default()), Pickup: reminderPickupReader{source: pickup},
		Instance: reminderTimetableReader{source: r.Timetable}, Room: reminderRoomReader{source: rooms},
		Student: reminderStudentReader{source: r.Student}, Person: reminderPersonReader{source: r.Person}, Supervision: reminderSupervisionReader{source: groups.Active, presence: newStudentPresence(db, slog.Default())},
		Visits: reminderVisitReader{source: newStudentPresence(db, slog.Default())}, Logger: slog.Default(), BulkSupervision: reminderBulkSupervisionReader{source: r.GroupSupervisor}, BulkInstanceStaff: reminderTimetableReader{source: r.Timetable},
	})
	return RemindersTestModule{Reminders: service, Settings: settings.Settings, UserContext: groups.UserContext}, nil
}
