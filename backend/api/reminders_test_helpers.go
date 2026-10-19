//go:build testhooks

package api

import (
	"context"
	"log/slog"
	"time"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	reminderCompose "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/compose"
	reminderPorts "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/ports"
)

// RemindersTestModule is the reminder query over the test database.
type RemindersTestModule struct {
	Reminders reminder.Query
}

// NewRemindersTestModule composes the reminder query and returns the tenant
// settings it reads beside it.
func NewRemindersTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, clocks ...func() time.Time) (RemindersTestModule, owners.SettingsTestModule, error) {
	rooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		return RemindersTestModule{}, owners.SettingsTestModule{}, err
	}
	r, err := NewTimetableTestRepositories(db)
	if err != nil {
		return RemindersTestModule{}, owners.SettingsTestModule{}, err
	}
	members := owners.MustNewMembershipTestRepositories(db)
	presence, err := NewTimetablePresenceRows(db, r, members)
	if err != nil {
		return RemindersTestModule{}, owners.SettingsTestModule{}, err
	}
	approvedOfferings, err := NewApprovedOfferingTestProjection(db, r.Enrollment())
	if err != nil {
		return RemindersTestModule{}, owners.SettingsTestModule{}, err
	}
	_, groupPresence, _, groupCaller, err := NewGroupsTestModule(db, unit)
	if err != nil {
		return RemindersTestModule{}, owners.SettingsTestModule{}, err
	}
	settings, err := owners.NewSettingsTestModule(db, unit)
	if err != nil {
		return RemindersTestModule{}, owners.SettingsTestModule{}, err
	}
	students, err := owners.NewPeopleDirectory(db)
	if err != nil {
		return RemindersTestModule{}, owners.SettingsTestModule{}, err
	}
	carePlan, err := owners.NewCarePlan(db, students, r.InstanceStudent)
	if err != nil {
		return RemindersTestModule{}, owners.SettingsTestModule{}, err
	}
	baseline, err := careplanCompose.NewPickupBaselines(carePlan, approvedOfferings, func(ctx context.Context) (bool, error) {
		return settings.Settings.ResolveBool(ctx, configModels.KeyEnrollmentBookingsAuthoritative)
	})
	if err != nil {
		return RemindersTestModule{}, owners.SettingsTestModule{}, err
	}
	autoExcusal, err := careplanCompose.NewPickupAutoExcusal(careplanCompose.PickupExcusalDependencies{
		DB: db.Pool(), Records: carePlan, Baselines: baseline, Blocks: owners.NewLoggedStudentPresence(db, slog.Default()), Preview: owners.NewPickupExcusalTimetable(r.Timetable, owners.NewPartialAbsencePreview(db)),
	})
	if err != nil {
		return RemindersTestModule{}, owners.SettingsTestModule{}, err
	}
	pickupStudents, err := owners.PickupPlanStudents(students)
	if err != nil {
		return RemindersTestModule{}, owners.SettingsTestModule{}, err
	}
	pickup, err := careplanCompose.NewPickupSchedules(db.Pool(), carePlan, baseline, autoExcusal, pickupStudents, slog.Default())
	if err != nil {
		return RemindersTestModule{}, owners.SettingsTestModule{}, err
	}
	service := reminderCompose.NewQuery(reminderPorts.QueryDependencies{
		Clock:        owners.ReminderClock(clocks...),
		CurrentStaff: owners.ReminderStaffIdentity(groupCaller),
		Settings:     owners.ReminderSettings{SettingsService: settings.Settings}, Attendance: owners.NewLoggedStudentPresence(db, slog.Default()), Pickup: owners.ReminderPickupReader{Source: pickup},
		Instance: owners.ReminderTimetableReader{Source: r.Timetable}, Room: owners.ReminderRoomReader{Source: rooms},
		Student: owners.ReminderStudentReader{Source: members.Student}, Person: owners.ReminderPersonReader{Source: members.Person}, Supervision: owners.ReminderSupervisionReader{Source: groupPresence.Active, Presence: owners.NewLoggedStudentPresence(db, slog.Default())},
		Visits: owners.ReminderVisitReader{Source: owners.NewLoggedStudentPresence(db, slog.Default())}, Logger: slog.Default(), BulkSupervision: owners.ReminderBulkSupervisionReader{Source: presence.GroupSupervisor}, BulkInstanceStaff: owners.ReminderTimetableReader{Source: r.Timetable},
	})
	return RemindersTestModule{Reminders: service}, settings, nil
}
