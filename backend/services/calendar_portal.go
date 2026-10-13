package services

import (
	"context"
	"errors"
	"log/slog"

	appointmentcap "github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/emailbranding"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/emailoutbox"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/email"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	platformModels "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	calendarPortal "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
	calendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/compose"
	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
)

type CalendarDependencies struct {
	CalendarFacts
	Appointments appointmentcap.Capability
	// RoomRepo resolves room names for timetable events in one batch per
	// window (#2078). Optional: nil leaves Location empty instead of failing
	// the whole calendar, mirroring StaffShiftRepo/ShiftTypeRepo below.
	UserContext identityaccess.CallerIdentities
	// Runtime is the tenant runtime over the root database
	// (calendarCompose.NewTenantRuntime); nil runs without a database.
	Runtime          *calendarCompose.TenantRuntime
	CalendarRenderer calendarPortal.CalendarRenderer

	// Notification dependencies (all optional — nil disables e-mail; the in-app
	// calendar is unaffected).
	Outbox                 CalendarOutbox
	PushOutbox             CalendarPushOutbox
	Settings               CalendarLogoResolver
	CalDAVPolicy           calendarPortal.CalDAVPolicy
	StaffFeedTombstoneRepo schoolcalendar.FeedHistory
	ParentsURL             string
	FrontendURL            string
	CalDAVURL              string
	// Today pins the feeds' calendar day; nil uses the real day.
	Today func() appointmentcap.Date

	// Notifier and Preferences drive the guardian push/in-app notification that
	// accompanies the appointment e-mails (#1671). Both optional and both
	// required together: without consent there is nobody to address, so a nil
	// Preferences disables the push rather than broadcasting past consent.
	Notifier notifications.Service
	// ReminderNotifier waits for Web Push acceptance so the scheduler can retry
	// a transient failure without permanently claiming the reminder delivery.
	ReminderNotifier notifications.SynchronousService
	Preferences      notifications.PreferenceService

	// Logger is nil-safe (see service.logger()); notification dispatch is
	// fire-and-forget and reports its failures here instead of to the caller.
	Logger *slog.Logger
}

type CalendarOutbox interface {
	Enqueue(context.Context, emailoutbox.EnqueueRequest) (*emailoutbox.Enqueued, error)
	CancelPendingByRelatedEntity(context.Context, string, int64, string) (int64, error)
}
type CalendarPushOutbox interface {
	CancelPendingByRelatedEntity(context.Context, string, int64, string) (int64, error)
}
type CalendarLogoResolver interface {
	GetLoginImageURL(context.Context, int64) (string, error)
}

// NewCalendarPortal adapts the existing composition's dependencies to the
// calendar's narrow fact ports. It does not build a legacy factory.
func NewCalendarPortal(d CalendarDependencies) calendarCompose.Application {
	cfg := NewCalendarFacts(d.CalendarFacts)
	cfg.MissingRecord = func(err error) bool { return errors.Is(err, transactionruntime.ErrNoRows) }
	cfg.MissingRecordCause = transactionruntime.ErrNoRows
	cfg.Appointments = d.Appointments
	cfg.CalendarRenderer = d.CalendarRenderer
	cfg.PushOutbox = d.PushOutbox
	cfg.Settings = d.Settings
	cfg.CalDAVPolicy = d.CalDAVPolicy
	cfg.StaffFeedTombstoneRepo = d.StaffFeedTombstoneRepo
	cfg.ParentsURL = d.ParentsURL
	cfg.FrontendURL = d.FrontendURL
	cfg.CalDAVURL = d.CalDAVURL
	cfg.Today = d.Today
	cfg.Logger = d.Logger
	cfg.Preferences = d.Preferences
	runtime := d.Runtime
	if runtime == nil {
		runtime = calendarCompose.NewTenantRuntime(nil)
	}
	cfg.Runtime = runtime
	cfg.NotificationCopy = calendarNotificationCopy
	cfg.MotoLogoURL = emailbranding.MotoLogoURL
	cfg.SchoolLogoURL = emailbranding.SchoolLogoURL
	cfg.PushSuppressed = calendarPushSuppressed
	cfg.NoDelivery = func(err error) bool {
		return errors.Is(err, notifications.ErrNoWebPushSubscribers) || calendarPushSuppressed(err)
	}
	if d.UserContext != nil {
		cfg.UserContext = calendarIdentityPort{d.UserContext}
	}
	if d.Outbox != nil {
		cfg.Outbox = calendarOutboxPort{CalendarOutbox: d.Outbox, tenantID: runtime.TenantID}
	}
	if d.Notifier != nil {
		cfg.Notifier = calendarNotificationPort{d.Notifier}
	}
	if d.ReminderNotifier != nil {
		cfg.ReminderNotifier = calendarSynchronousNotificationPort{d.ReminderNotifier}
	}
	return calendarCompose.New(cfg)
}

func calendarPushSuppressed(err error) bool {
	return errors.Is(err, notifications.ErrDisabled) || errors.Is(err, notifications.ErrOutsideActiveWindow)
}

func calendarNotificationCopy(kind, locale string) (string, string) {
	notificationKind := notifications.ParentAppointmentPublished
	switch kind {
	case platformModels.EmailKindAppointmentUpdated:
		notificationKind = notifications.ParentAppointmentUpdated
	case platformModels.EmailKindAppointmentCancelled:
		notificationKind = notifications.ParentAppointmentCancelled
	case platformModels.EmailKindAppointmentReminder:
		notificationKind = notifications.ParentAppointmentReminder
	}
	return notifications.ParentAppointmentCopy(locale, notificationKind)
}

type calendarIdentityPort struct {
	source identityaccess.CallerIdentities
}

func (p calendarIdentityPort) GetCurrentStaff(ctx context.Context) (*calendarCompose.Staff, error) {
	staffID, err := p.source.StaffID(ctx)
	if err != nil {
		return nil, err
	}
	return &calendarCompose.Staff{ID: staffID}, nil
}
func (p calendarIdentityPort) GetCurrentUser(ctx context.Context) (*calendarCompose.Account, error) {
	value, err := p.source.Account(ctx)
	if err != nil {
		return nil, err
	}
	return &calendarCompose.Account{ID: value.ID, Email: value.Email, Active: value.Active}, nil
}
func (p calendarIdentityPort) MissingUser(err error) bool {
	return errors.Is(err, identityaccess.ErrCallerNotAuthenticated) || errors.Is(err, identityaccess.ErrCallerNotFound)
}
func (p calendarIdentityPort) MissingStaff(err error) bool {
	return p.MissingUser(err) || errors.Is(err, identityaccess.ErrCallerNotLinkedToPerson) || errors.Is(err, identityaccess.ErrCallerNotLinkedToStaff)
}

type calendarOutboxPort struct {
	CalendarOutbox
	tenantID func(context.Context) int64
}

func (p calendarOutboxPort) Enqueue(ctx context.Context, req calendarCompose.EnqueueRequest) (*calendarCompose.EmailOutbox, error) {
	value, err := p.CalendarOutbox.Enqueue(ctx, emailoutbox.EnqueueRequest{Kind: req.Kind, Payload: req.Payload, RelatedEntityType: req.RelatedEntityType, RelatedEntityID: req.RelatedEntityID, IdempotencyKey: req.IdempotencyKey})
	if value == nil {
		return nil, err
	}
	return &calendarCompose.EmailOutbox{ID: value.ID, TenantID: p.tenantID(ctx), Kind: req.Kind, Payload: req.Payload}, err
}
func calendarOutbox(value *emailoutbox.Intent) *calendarCompose.EmailOutbox {
	if value == nil {
		return nil
	}
	return &calendarCompose.EmailOutbox{ID: value.ID, TenantID: value.TenantID, Kind: value.Kind, Payload: value.Payload}
}
func calendarNotification(value calendarCompose.Notification) notifications.Event {
	return notifications.Event{Type: value.Type, IdempotencyKey: value.IdempotencyKey, RelatedType: value.RelatedType, RelatedID: value.RelatedID, Title: value.Title, Body: value.Body, DeepLink: value.DeepLink, Priority: value.Priority,
		Audience: notifications.Audience{TenantID: value.Audience.TenantID, Scope: notifications.AudienceScope(value.Audience.Scope), GuardianAccountIDs: value.Audience.GuardianAccountIDs, StudentIDs: value.Audience.StudentIDs}}
}

type calendarNotificationPort struct{ source notifications.Service }

func (p calendarNotificationPort) Notify(ctx context.Context, event calendarCompose.Notification) error {
	return p.source.Notify(ctx, calendarNotification(event))
}

type calendarSynchronousNotificationPort struct {
	source notifications.SynchronousService
}

func (p calendarSynchronousNotificationPort) NotifySynchronously(ctx context.Context, event calendarCompose.Notification) error {
	return p.source.NotifySynchronously(ctx, calendarNotification(event))
}

type CalendarEmailDependencies struct {
	DefaultFrom email.Email
	// Runtime re-checks the authorization scope a row carries before it
	// renders (calendarCompose.NewTenantRuntime); nil skips the re-check.
	Runtime   *calendarCompose.TenantRuntime
	Guardians interface {
		FilterAccountsWithStudentAccess(context.Context, []int64, []int64, int64, string) ([]int64, error)
	}
}

func NewCalendarAppointmentRenderer(d CalendarEmailDependencies) func(context.Context, *emailoutbox.Intent) (*email.Message, error) {
	cfg := calendarCompose.EmailConfig{Guardians: d.Guardians, RenderCancelled: emailoutbox.ErrRenderCancelled}
	if d.Runtime != nil {
		cfg.Runtime = d.Runtime
	}
	render := calendarCompose.NewAppointmentRenderer(cfg)
	return func(ctx context.Context, row *emailoutbox.Intent) (*email.Message, error) {
		value, err := render(ctx, calendarOutbox(row))
		if err != nil {
			return nil, err
		}
		return &email.Message{From: d.DefaultFrom, To: email.NewEmail("", value.To), Subject: value.Subject, Template: value.Template, Content: value.Content}, nil
	}
}
