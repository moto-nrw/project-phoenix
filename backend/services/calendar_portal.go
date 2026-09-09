package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	appointmentcap "github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/emailbranding"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	calendarPortal "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
	calendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/compose"
	platformService "github.com/moto-nrw/project-phoenix/services/platform"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type CalendarDependencies struct {
	repositories.CalendarFacts
	Appointments appointmentcap.Capability
	// RoomRepo resolves room names for timetable events in one batch per
	// window (#2078). Optional: nil leaves Location empty instead of failing
	// the whole calendar, mirroring StaffShiftRepo/ShiftTypeRepo below.
	UserContext      usercontext.UserContextService
	DB               *bun.DB
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
	Enqueue(context.Context, platformService.EnqueueRequest) (*platformModels.EmailOutbox, error)
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
	cfg := repositories.NewCalendarFacts(d.CalendarFacts)
	cfg.MissingRecord = func(err error) bool { return errors.Is(err, sql.ErrNoRows) }
	cfg.MissingRecordCause = sql.ErrNoRows
	cfg.Appointments = d.Appointments
	cfg.CalendarRenderer = d.CalendarRenderer
	cfg.PushOutbox = d.PushOutbox
	cfg.Settings = d.Settings
	cfg.CalDAVPolicy = d.CalDAVPolicy
	cfg.StaffFeedTombstoneRepo = d.StaffFeedTombstoneRepo
	cfg.ParentsURL = d.ParentsURL
	cfg.FrontendURL = d.FrontendURL
	cfg.CalDAVURL = d.CalDAVURL
	cfg.Logger = d.Logger
	cfg.Preferences = d.Preferences
	cfg.Runtime = calendarPortalRuntime{db: d.DB}
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
		cfg.Outbox = calendarOutboxPort{d.Outbox}
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

type calendarPortalRuntime struct{ db *bun.DB }

func (r calendarPortalRuntime) TenantID(ctx context.Context) int64 { return tenant.FromContext(ctx) }
func (r calendarPortalRuntime) WithinAppointmentWrite(ctx context.Context, command func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return command(ctx)
	}
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return fmt.Errorf("calendar appointment command: %w", err)
	}
	return tenant.WithinTenant(ctx, tenantID, command)
}
func (r calendarPortalRuntime) WithinTenant(ctx context.Context, tenantID int64, command func(context.Context) error) error {
	return tenant.WithTenantTx(ctx, r.db, tenantID, func(txCtx context.Context, _ bun.Tx) error { return command(txCtx) })
}
func (r calendarPortalRuntime) WithinAdmin(ctx context.Context, command func(context.Context) error) error {
	return tenant.WithAdminTx(ctx, r.db, func(txCtx context.Context, _ bun.Tx) error { return command(txCtx) })
}
func (r calendarPortalRuntime) AfterCommit(ctx context.Context, fn func()) {
	tenant.RegisterAfterCommit(ctx, fn)
}
func (r calendarPortalRuntime) WithoutTransactionAndHooks(ctx context.Context) context.Context {
	return tenant.ContextWithoutAfterCommitHooks(tenant.ContextWithoutTransaction(ctx))
}

type calendarIdentityPort struct {
	source usercontext.UserContextService
}

func (p calendarIdentityPort) GetCurrentStaff(ctx context.Context) (*calendarCompose.Staff, error) {
	value, err := p.source.GetCurrentStaff(ctx)
	if value == nil {
		return nil, err
	}
	person := value.Person
	var mapped *calendarCompose.Person
	if person != nil {
		mapped = &calendarCompose.Person{ID: person.ID, FirstName: person.FirstName, LastName: person.LastName}
	}
	return &calendarCompose.Staff{ID: value.ID, Person: mapped}, err
}
func (p calendarIdentityPort) GetCurrentUser(ctx context.Context) (*calendarCompose.Account, error) {
	value, err := p.source.GetCurrentUser(ctx)
	if value == nil {
		return nil, err
	}
	return &calendarCompose.Account{ID: value.ID, Email: value.Email, Active: value.IsActive(), CalendarFeedToken: value.CalendarFeedToken}, err
}
func (p calendarIdentityPort) MissingUser(err error) bool {
	return errors.Is(err, usercontext.ErrUserNotAuthenticated) || errors.Is(err, usercontext.ErrUserNotFound)
}
func (p calendarIdentityPort) MissingStaff(err error) bool {
	return p.MissingUser(err) || errors.Is(err, usercontext.ErrUserNotLinkedToPerson) || errors.Is(err, usercontext.ErrUserNotLinkedToStaff)
}

type calendarOutboxPort struct{ CalendarOutbox }

func (p calendarOutboxPort) Enqueue(ctx context.Context, req calendarCompose.EnqueueRequest) (*calendarCompose.EmailOutbox, error) {
	value, err := p.CalendarOutbox.Enqueue(ctx, platformService.EnqueueRequest{Kind: req.Kind, Payload: req.Payload, RelatedEntityType: req.RelatedEntityType, RelatedEntityID: req.RelatedEntityID, IdempotencyKey: req.IdempotencyKey})
	return calendarOutbox(value), err
}
func calendarOutbox(value *platformModels.EmailOutbox) *calendarCompose.EmailOutbox {
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
	DB          *bun.DB
	Guardians   interface {
		FilterAccountsWithStudentAccess(context.Context, []int64, []int64, int64, string) ([]int64, error)
	}
}

func NewCalendarAppointmentRenderer(d CalendarEmailDependencies) func(context.Context, *platformModels.EmailOutbox) (*email.Message, error) {
	cfg := calendarCompose.EmailConfig{Guardians: d.Guardians, RenderCancelled: platformService.ErrRenderCancelled}
	if d.DB != nil {
		cfg.Runtime = calendarPortalRuntime{d.DB}
	}
	render := calendarCompose.NewAppointmentRenderer(cfg)
	return func(ctx context.Context, row *platformModels.EmailOutbox) (*email.Message, error) {
		value, err := render(ctx, calendarOutbox(row))
		if err != nil {
			return nil, err
		}
		return &email.Message{From: d.DefaultFrom, To: email.NewEmail("", value.To), Subject: value.Subject, Template: value.Template, Content: value.Content}, nil
	}
}
