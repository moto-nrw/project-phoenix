package services

import (
	"log/slog"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	calendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type CalendarTestModule struct{ Calendar calendarCompose.Application }

func NewCalendarTestModule(db *bun.DB, unit tenant.UnitOfWork) (CalendarTestModule, error) {
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
	cfg := currentFactoryConfig()
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
		DB:                     db,
		CalendarRenderer:       schoolCalendarRendererAdapter{renderer: schoolCalendar},
		Outbox:                 delivery.EmailOutbox,
		PushOutbox:             durablePushAdapter{module: delivery.Delivery},
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
