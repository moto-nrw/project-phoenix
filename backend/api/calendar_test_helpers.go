//go:build testhooks

package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"

	"github.com/go-chi/chi/v5"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	email "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
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
	command, err := auditCompose.NewCommand(owners.NewTestAuditStore(db), func(auditCompose.AppendObservation) {})
	if err != nil {
		return CalendarTestModule{}, err
	}
	repos, err := NewEnrollmentTestRepositories(db, command)
	if err != nil {
		return CalendarTestModule{}, err
	}
	reposTimetable := NewEnrollmentTimetableRows(db)
	reposRooms := MustNewTimetableRoomRows(db)
	reposSchools := NewOrganizationTestRows(db)
	members := owners.MustNewMembershipTestRepositories(db)
	parents, err := NewParentRouteTestRepositories(db, members)
	if err != nil {
		return CalendarTestModule{}, err
	}
	appointments, err := NewAppointments(db)
	if err != nil {
		return CalendarTestModule{}, err
	}
	schoolCalendar, err := owners.NewSchoolCalendar(db)
	if err != nil {
		return CalendarTestModule{}, err
	}
	feeds := NewCalendarFeedTestRepositories(db)
	identity, err := NewUserContextTestModule(db, unit)
	if err != nil {
		return CalendarTestModule{}, err
	}
	settings, err := owners.NewSettingsTestModule(db, unit)
	if err != nil {
		return CalendarTestModule{}, err
	}
	delivery, err := NewDeliveryTestModule(db, unit)
	if err != nil {
		return CalendarTestModule{}, err
	}
	logger := slog.Default()
	cfg := CurrentServiceConfig()
	calendarConfig := owners.NewCalendarFacts(repos.Staff, repos.Student, owners.NewCalendarGuardianDirectory(repos.GuardianProfile, owners.MustNewPeopleDirectory(db)),
		repos.StudentGuardian, parents.ParentChildLinks(), owners.NewSchoolStructureSuiteRepositories(db).Group)
	calendarConfig = owners.BindCalendarScheduleFacts(calendarConfig, reposTimetable.InstanceStaff, reposTimetable.ActivityInstance.Reads, reposRooms.Room, reposTimetable.StaffShift, reposTimetable.ShiftType)
	calendarConfig = owners.BindCalendarFeedFacts(calendarConfig, reposSchools.School, feeds.ParentFeed, feeds.StaffFeed, repos.Person)
	calendarConfig = owners.BindCalendarCaller(calendarConfig, identity.Caller())
	calendarConfig.Now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	calendarConfig.Appointments = appointments
	calendarConfig.Runtime = calendarCompose.NewTenantRuntime(db.Pool())
	calendarConfig.CalendarRenderer = calendarCompose.NewCalendarRenderer(schoolCalendar)
	calendarConfig.Outbox = owners.NewCalendarOutbox(delivery.EmailOutbox)
	calendarConfig.PushOutbox = deliveryCompose.NewDurablePushOutbox(delivery.Runtime.Module)
	calendarConfig.Settings = settings.Settings
	calendarConfig.StaffFeedTombstoneRepo = NewCalendarFeedTestTombstones(db)
	calendarConfig.ParentsURL = cfg.ParentsURL
	calendarConfig.FrontendURL = cfg.FrontendURL
	calendarConfig.Notifier = owners.NewCalendarNotifier(delivery.Notifications)
	calendarConfig.ReminderNotifier = owners.NewCalendarReminderNotifier(delivery.Notifications)
	calendarConfig.Preferences = delivery.NotificationPreferences
	calendarConfig.Logger = logger.With("service", "calendar")
	calendarSvc := owners.NewCalendarPortal(calendarConfig)

	return CalendarTestModule{Calendar: calendarSvc}, nil
}

// CalendarNoPushAudience is the error the root's Delivery binding of the
// calendar reports when no guardian device can take a push; the calendar
// treats it as nothing delivered, not as a failure.
func CalendarNoPushAudience() error { return deliveryCompose.ErrNoWebPushSubscribers }

// RenderCalendarMailForTest renders one queued calendar mail of the tenant
// the way the Delivery outbox worker does; rendered reports a message.
func RenderCalendarMailForTest(ctx context.Context, runtime *calendarCompose.TenantRuntime, guardians owners.CalendarMailGuardians, tenantID int64, kind string, payload map[string]any) (rendered bool, err error) {
	message, err := owners.NewCalendarAppointmentRenderer(email.Email{}, runtime, guardians)(ctx, &deliveryCompose.MailIntent{TenantID: tenantID, Kind: kind, Payload: payload})
	return message != nil, err
}

// CalendarRenderCancelled is the error a calendar mail render reports when
// the mail may never go out, so Delivery stops retrying it.
func CalendarRenderCancelled() error { return deliveryCompose.ErrRenderCancelled }

// CalendarOutboxNotifierForTest exposes the enqueue boundary without granting
// calendar tests a foreign module dependency.
func CalendarOutboxNotifierForTest(module DeliveryTestModule) fixtureCalendarNotifier {
	return func(ctx context.Context, event calendarCompose.Notification) error {
		payload, err := json.Marshal(event)
		if err != nil {
			return err
		}
		_, err = module.Runtime.Module.EnqueueDeferredPush(ctx, deliveryContract.DeferredPushIntent{TenantID: event.Audience.TenantID, Template: event.Type, IdempotencyKey: event.IdempotencyKey, Related: deliveryContract.RelatedEntity{Type: event.RelatedType, ID: event.RelatedID}, Payload: payload})
		return err
	}
}

type fixtureCalendarNotifier func(context.Context, calendarCompose.Notification) error

func (notify fixtureCalendarNotifier) Notify(ctx context.Context, event calendarCompose.Notification) error {
	return notify(ctx, event)
}
