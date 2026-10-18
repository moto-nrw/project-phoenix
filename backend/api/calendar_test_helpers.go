//go:build testhooks

package api

import (
	"context"
	"log/slog"

	"github.com/go-chi/chi/v5"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/email"
	calendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	parentAPI "github.com/moto-nrw/project-phoenix/workflows/parentportal/http"
)

type CalendarTestModule struct{ Calendar calendarCompose.Application }

// ParentCalendarRouter mounts the guardian portal routes with only the
// module's calendar wired, so the calendar end-to-end suite drives the
// staff-to-parent flow without naming the portal adapter (#2748).
func (m CalendarTestModule) ParentCalendarRouter() chi.Router {
	return parentAPI.NewResource(parentAPI.ResourceConfig{Calendar: m.Calendar}).Router(ParentSessionChain)
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
		Outbox:                 newCalendarOutbox(delivery.EmailOutbox),
		PushOutbox:             deliveryCompose.NewDurablePushOutbox(delivery.Delivery),
		Settings:               settings.Settings,
		StaffFeedTombstoneRepo: feeds.Tombstone,
		ParentsURL:             cfg.ParentsURL,
		FrontendURL:            cfg.FrontendURL,
		Notifier:               newCalendarNotifier(delivery.Notifications),
		ReminderNotifier:       newCalendarReminderNotifier(delivery.Notifications),
		Preferences:            delivery.NotificationPreferences,
		Logger:                 logger.With("service", "calendar"),
	})

	return CalendarTestModule{Calendar: calendarSvc}, nil
}

// CalendarNoPushAudience is the error the root's Delivery binding of the
// calendar reports when no guardian device can take a push; the calendar
// treats it as nothing delivered, not as a failure.
func CalendarNoPushAudience() error { return deliveryCompose.ErrNoWebPushSubscribers }

// RenderCalendarMailForTest renders one queued calendar mail of the tenant
// the way the Delivery outbox worker does; rendered reports a message.
func RenderCalendarMailForTest(ctx context.Context, runtime *calendarCompose.TenantRuntime, guardians CalendarMailGuardians, tenantID int64, kind string, payload map[string]any) (rendered bool, err error) {
	message, err := NewCalendarAppointmentRenderer(email.Email{}, runtime, guardians)(ctx, &deliveryCompose.MailIntent{TenantID: tenantID, Kind: kind, Payload: payload})
	return message != nil, err
}

// CalendarRenderCancelled is the error a calendar mail render reports when
// the mail may never go out, so Delivery stops retrying it.
func CalendarRenderCancelled() error { return deliveryCompose.ErrRenderCancelled }
