package services

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	auditService "github.com/moto-nrw/project-phoenix/services/audit"
	calendarService "github.com/moto-nrw/project-phoenix/services/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type CalendarTestModule struct{ Calendar calendarService.FullService }

func NewCalendarTestModule(db *bun.DB, unit tenant.UnitOfWork) (CalendarTestModule, error) {
	command, err := auditService.NewCommand(repositories.NewTestAuditStore(db), func(auditService.AppendObservation) {})
	if err != nil {
		return CalendarTestModule{}, err
	}
	repos, err := repositories.NewEnrollmentTestRepositories(db, command)
	if err != nil {
		return CalendarTestModule{}, err
	}
	parents, err := repositories.NewParentRouteTestRepositories(db)
	if err != nil {
		return CalendarTestModule{}, err
	}
	appointments, err := repositories.NewAppointments(db)
	if err != nil {
		return CalendarTestModule{}, err
	}
	schoolCalendar, err := repositories.NewSchoolCalendar(db)
	if err != nil {
		return CalendarTestModule{}, err
	}
	feeds := repositories.NewCalendarFeedTestRepositories(db)
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
	calendarSvc := calendarService.NewService(calendarService.Config{
		Appointments:           appointments,
		StaffRepo:              repos.Staff,
		StudentRepo:            repos.Student,
		GuardianProfileRepo:    repos.GuardianProfile,
		StudentGuardianRepo:    repos.StudentGuardian,
		ChildRepo:              parents.ParentChild,
		GroupRepo:              repos.Group,
		InstanceStaffRepo:      repos.InstanceStaff,
		ActivityInstanceRepo:   repos.ActivityInstance,
		RoomRepo:               repos.Room,
		StaffShiftRepo:         repos.StaffShift,
		ShiftTypeRepo:          repos.ShiftType,
		UserContext:            identity.UserContext,
		DB:                     db,
		CalendarRenderer:       schoolCalendarRendererAdapter{renderer: schoolCalendar},
		Outbox:                 delivery.EmailOutbox,
		PushOutbox:             durablePushAdapter{module: delivery.Delivery},
		SchoolRepo:             repos.School,
		Settings:               settings.Settings,
		AccountRepo:            repos.Account,
		StaffFeedRepo:          feeds.StaffFeed,
		StaffFeedTombstoneRepo: feeds.Tombstone,
		PersonRepo:             repos.Person,
		ParentsURL:             cfg.ParentsURL,
		FrontendURL:            cfg.FrontendURL,
		Notifier:               delivery.Notifications,
		ReminderNotifier:       delivery.Notifications,
		Preferences:            delivery.NotificationPreferences,
		Logger:                 logger.With("service", "calendar"),
	})

	return CalendarTestModule{Calendar: calendarSvc}, nil
}
