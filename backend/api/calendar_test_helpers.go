//go:build testhooks

package api

import (
	"log/slog"

	"github.com/go-chi/chi/v5"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	parentAPI "github.com/moto-nrw/project-phoenix/modules/careplan/inbound/parent"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	calendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type CalendarTestModule struct{ Calendar calendarCompose.Application }

// ParentCalendarRouter mounts the guardian portal routes with only the
// module's calendar wired, so the calendar end-to-end suite drives the
// staff-to-parent flow without naming the portal adapter (#2748).
func (m CalendarTestModule) ParentCalendarRouter() chi.Router {
	return parentAPI.NewResource(parentAPI.ResourceConfig{Calendar: m.Calendar}).Router()
}

func NewCalendarTestModule(db transactionruntime.Database, unit tenant.UnitOfWork) (CalendarTestModule, error) {
	command, err := auditCompose.NewCommand(NewTestAuditStore(db), func(auditCompose.AppendObservation) {})
	if err != nil {
		return CalendarTestModule{}, err
	}
	repos, err := NewEnrollmentTestRepositories(db, command)
	if err != nil {
		return CalendarTestModule{}, err
	}
	parents, err := NewParentRouteTestRepositories(db, MustNewMembershipTestRepositories(db))
	if err != nil {
		return CalendarTestModule{}, err
	}
	appointments, err := NewAppointments(db)
	if err != nil {
		return CalendarTestModule{}, err
	}
	schoolCalendar, err := NewSchoolCalendar(db)
	if err != nil {
		return CalendarTestModule{}, err
	}
	feeds := NewCalendarFeedTestRepositories(db)
	identity, err := NewUserContextTestModule(db, unit)
	if err != nil {
		return CalendarTestModule{}, err
	}
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return CalendarTestModule{}, err
	}
	delivery, err := NewDeliveryTestModule(db, unit)
	if err != nil {
		return CalendarTestModule{}, err
	}
	logger := slog.Default()
	cfg := CurrentServiceConfig()
	calendarSvc := NewCalendarPortal(CalendarDependencies{
		CalendarFacts: CalendarFacts{
			StaffRepo:            repos.Staff,
			StudentRepo:          repos.Student,
			GuardianProfileRepo:  NewCalendarGuardianDirectory(repos.GuardianProfile, MustNewPeopleDirectory(db)),
			StudentGuardianRepo:  repos.StudentGuardian,
			ChildRepo:            parents.ParentChildLinks(),
			GroupRepo:            repos.Group,
			InstanceStaffRepo:    repos.InstanceStaff,
			ActivityInstanceRepo: repos.ActivityInstance,
			RoomRepo:             repos.Room,
			StaffShiftRepo:       repos.StaffShift,
			ShiftTypeRepo:        repos.ShiftType,
			SchoolRepo:           repos.School,
			AccountRepo:          feeds.ParentFeed,
			StaffFeedRepo:        feeds.StaffFeed,
			PersonRepo:           repos.Person,
		},
		Appointments:           appointments,
		UserContext:            identity.UserContext.Caller(),
		Runtime:                calendarCompose.NewTenantRuntime(db.Pool()),
		CalendarRenderer:       calendarCompose.NewCalendarRenderer(schoolCalendar),
		Outbox:                 delivery.EmailOutbox,
		PushOutbox:             deliveryCompose.NewDurablePushOutbox(delivery.Delivery),
		Settings:               settings.Settings,
		StaffFeedTombstoneRepo: feeds.Tombstone,
		ParentsURL:             cfg.ParentsURL,
		FrontendURL:            cfg.FrontendURL,
		Notifier:               delivery.Notifications,
		ReminderNotifier:       delivery.Notifications,
		Preferences:            delivery.NotificationPreferences,
		Logger:                 logger.With("service", "calendar"),
	})

	return CalendarTestModule{Calendar: calendarSvc}, nil
}
