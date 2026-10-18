package api

import (
	"context"
	"errors"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/email"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	calendarService "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
	calendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// NewCalendarPortal composes the staff and parent calendars over cfg, whose
// fact ports NewCalendarFacts and the Bind functions fill and whose other
// values the caller sets (all notification dependencies are optional: nil
// disables e-mail, the in-app calendar is unaffected). It binds the
// root's missing-record, notification-copy, logo and push rules; without a
// Runtime it runs without a database. It does not build a legacy factory.
func NewCalendarPortal(cfg calendarCompose.Config) calendarCompose.Application {
	cfg.MissingRecord = func(err error) bool { return errors.Is(err, transactionruntime.ErrNoRows) }
	cfg.MissingRecordCause = transactionruntime.ErrNoRows
	if cfg.Runtime == nil {
		cfg.Runtime = calendarCompose.NewTenantRuntime(nil)
	}
	cfg.NotificationCopy = calendarNotificationCopy
	cfg.MotoLogoURL = deliveryCompose.MotoLogoURL
	cfg.SchoolLogoURL = deliveryCompose.SchoolLogoURL
	cfg.PushSuppressed = calendarPushSuppressed
	cfg.NoDelivery = func(err error) bool {
		return errors.Is(err, deliveryCompose.ErrNoWebPushSubscribers) || calendarPushSuppressed(err)
	}
	return calendarCompose.New(cfg)
}

// BindCalendarCaller binds the calendar's identity port to Identity &
// Access's caller identities; nil leaves it as it is.
func BindCalendarCaller(cfg calendarCompose.Config, caller identityaccess.CallerIdentities) calendarCompose.Config {
	if caller != nil {
		cfg.UserContext = calendarIdentityPort{caller}
	}
	return cfg
}

// NewCalendarCalDAVPolicy decides whether CalDAV is enabled by resolving
// calendar.caldav_enabled through settings.
func NewCalendarCalDAVPolicy(settings CalendarCalDAVSettings) calendarService.CalDAVPolicy {
	return calendarCalDAVPolicy{settings: settings}
}

// CalendarNotifier, CalendarReminderNotifier and CalendarPreferences are the
// calendar's guardian notification ports in its own terms; the root binds
// them to Delivery (newCalendarNotifier, newCalendarReminderNotifier).
type CalendarNotifier interface {
	Notify(context.Context, calendarCompose.Notification) error
}

type CalendarReminderNotifier interface {
	NotifySynchronously(context.Context, calendarCompose.Notification) error
}

type CalendarPreferences interface {
	FilterNotOptedOut(ctx context.Context, notificationType string, accountIDs []int64) ([]int64, error)
	FilterOptedIn(ctx context.Context, notificationType string, accountIDs []int64) ([]int64, error)
}

// calendarDeliveryOutbox is Delivery's mail outbox the calendar's mails go to.
type calendarDeliveryOutbox interface {
	Enqueue(context.Context, deliveryCompose.MailEnqueueRequest) (*deliveryCompose.MailEnqueued, error)
	CancelPendingByRelatedEntity(context.Context, string, int64, string) (int64, error)
}

// newCalendarOutbox binds the calendar's mail outbox to Delivery's; nil
// without an outbox.
func newCalendarOutbox(outbox calendarDeliveryOutbox) CalendarOutbox {
	if outbox == nil {
		return nil
	}
	return calendarOutboxPort{calendarDeliveryOutbox: outbox}
}

// newCalendarNotifier binds the calendar's guardian notification to
// Delivery's; nil without a notifier.
func newCalendarNotifier(source deliveryCompose.NotificationService) CalendarNotifier {
	if source == nil {
		return nil
	}
	return calendarNotificationPort{source}
}

// newCalendarReminderNotifier binds the calendar's reminder push to
// Delivery's synchronous notification; nil without a notifier.
func newCalendarReminderNotifier(source deliveryCompose.SynchronousNotificationService) CalendarReminderNotifier {
	if source == nil {
		return nil
	}
	return calendarSynchronousNotificationPort{source}
}

// calendarCalDAVPolicy reads whether CalDAV is enabled for the tenant.
type calendarCalDAVPolicy struct {
	settings CalendarCalDAVSettings
}

func (p calendarCalDAVPolicy) Enabled(ctx context.Context) (bool, error) {
	return p.settings.ResolveBool(ctx, configModels.KeyCalendarCalDAVEnabled)
}

func (p calendarCalDAVPolicy) EnabledForTenant(ctx context.Context, tenantID int64) (bool, error) {
	return p.settings.ResolveBoolForTenant(ctx, tenantID, configModels.KeyCalendarCalDAVEnabled)
}

func calendarPushSuppressed(err error) bool {
	return errors.Is(err, deliveryCompose.ErrNotificationsDisabled) || errors.Is(err, deliveryCompose.ErrOutsideNotificationWindow)
}

func calendarNotificationCopy(kind, locale string) (string, string) {
	notificationKind := deliveryCompose.ParentAppointmentPublished
	switch kind {
	case calendarService.EmailKindAppointmentUpdated:
		notificationKind = deliveryCompose.ParentAppointmentUpdated
	case calendarService.EmailKindAppointmentCancelled:
		notificationKind = deliveryCompose.ParentAppointmentCancelled
	case calendarService.EmailKindAppointmentReminder:
		notificationKind = deliveryCompose.ParentAppointmentReminder
	}
	return deliveryCompose.ParentAppointmentCopy(locale, notificationKind)
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
	calendarDeliveryOutbox
}

func (p calendarOutboxPort) Enqueue(ctx context.Context, req calendarCompose.EnqueueRequest) (*calendarCompose.EmailOutbox, error) {
	value, err := p.calendarDeliveryOutbox.Enqueue(ctx, deliveryCompose.MailEnqueueRequest{Kind: req.Kind, Payload: req.Payload, RelatedEntityType: req.RelatedEntityType, RelatedEntityID: req.RelatedEntityID, IdempotencyKey: req.IdempotencyKey})
	if value == nil {
		return nil, err
	}
	return &calendarCompose.EmailOutbox{ID: value.ID, TenantID: tenant.FromContext(ctx), Kind: req.Kind, Payload: req.Payload}, err
}
func calendarOutbox(value *deliveryCompose.MailIntent) *calendarCompose.EmailOutbox {
	if value == nil {
		return nil
	}
	return &calendarCompose.EmailOutbox{ID: value.ID, TenantID: value.TenantID, Kind: value.Kind, Payload: value.Payload}
}
func calendarNotification(value calendarCompose.Notification) deliveryCompose.NotificationEvent {
	return deliveryCompose.NotificationEvent{Type: value.Type, IdempotencyKey: value.IdempotencyKey, RelatedType: value.RelatedType, RelatedID: value.RelatedID, Title: value.Title, Body: value.Body, DeepLink: value.DeepLink, Priority: value.Priority,
		Audience: deliveryCompose.NotificationAudience{TenantID: value.Audience.TenantID, Scope: deliveryCompose.NotificationAudienceScope(value.Audience.Scope), GuardianAccountIDs: value.Audience.GuardianAccountIDs, StudentIDs: value.Audience.StudentIDs}}
}

type calendarNotificationPort struct {
	source deliveryCompose.NotificationService
}

func (p calendarNotificationPort) Notify(ctx context.Context, event calendarCompose.Notification) error {
	return p.source.Notify(ctx, calendarNotification(event))
}

type calendarSynchronousNotificationPort struct {
	source deliveryCompose.SynchronousNotificationService
}

func (p calendarSynchronousNotificationPort) NotifySynchronously(ctx context.Context, event calendarCompose.Notification) error {
	return p.source.NotifySynchronously(ctx, calendarNotification(event))
}

// CalendarMailGuardians re-checks which guardian accounts still reach the
// children a queued calendar mail names.
type CalendarMailGuardians interface {
	FilterAccountsWithStudentAccess(context.Context, []int64, []int64, int64, string) ([]int64, error)
}

// NewCalendarAppointmentRenderer renders the calendar's queued mails from
// defaultFrom. runtime re-checks the authorization scope a row carries
// before it renders (calendarCompose.NewTenantRuntime); nil skips the
// re-check. guardians answers the guardian access re-check.
func NewCalendarAppointmentRenderer(defaultFrom email.Email, runtime *calendarCompose.TenantRuntime, guardians CalendarMailGuardians) func(context.Context, *deliveryCompose.MailIntent) (*email.Message, error) {
	cfg := calendarCompose.EmailConfig{Guardians: guardians, RenderCancelled: deliveryCompose.ErrRenderCancelled}
	if runtime != nil {
		cfg.Runtime = runtime
	}
	render := calendarCompose.NewAppointmentRenderer(cfg)
	return func(ctx context.Context, row *deliveryCompose.MailIntent) (*email.Message, error) {
		value, err := render(ctx, calendarOutbox(row))
		if err != nil {
			return nil, err
		}
		return &email.Message{From: defaultFrom, To: email.NewEmail("", value.To), Subject: value.Subject, Template: value.Template, Content: value.Content}, nil
	}
}
