//go:build testhooks

package api

import (
	"context"
	"log/slog"
	"time"

	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
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
	members := MustNewMembershipTestRepositories(db)
	presence, err := NewTimetablePresenceRows(db, r, members)
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
		DB: db.Pool(), Records: carePlan, Baselines: baseline, Blocks: NewLoggedStudentPresence(db, slog.Default()), Preview: NewPickupExcusalTimetable(r.Timetable, NewPartialAbsencePreview(db)),
	})
	if err != nil {
		return RemindersTestModule{}, err
	}
	pickupStudents, err := PickupPlanStudents(students)
	if err != nil {
		return RemindersTestModule{}, err
	}
	pickup, err := careplanCompose.NewPickupSchedules(db.Pool(), carePlan, baseline, autoExcusal, pickupStudents, slog.Default())
	if err != nil {
		return RemindersTestModule{}, err
	}
	service := reminderCompose.NewQuery(reminderPorts.QueryDependencies{
		Clock:        ReminderClock(clocks...),
		CurrentStaff: ReminderStaffIdentity(groups.UserContext),
		Settings:     ReminderSettings{SettingsService: settings.Settings}, Attendance: NewLoggedStudentPresence(db, slog.Default()), Pickup: ReminderPickupReader{Source: pickup},
		Instance: ReminderTimetableReader{Source: r.Timetable}, Room: ReminderRoomReader{Source: rooms},
		Student: ReminderStudentReader{Source: members.Student}, Person: ReminderPersonReader{Source: members.Person}, Supervision: ReminderSupervisionReader{Source: groups.Active, Presence: NewLoggedStudentPresence(db, slog.Default())},
		Visits: ReminderVisitReader{Source: NewLoggedStudentPresence(db, slog.Default())}, Logger: slog.Default(), BulkSupervision: ReminderBulkSupervisionReader{Source: presence.GroupSupervisor}, BulkInstanceStaff: ReminderTimetableReader{Source: r.Timetable},
	})
	return RemindersTestModule{Reminders: service, Settings: settings.Settings, UserContext: groups.UserContext}, nil
}
