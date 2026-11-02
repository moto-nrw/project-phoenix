package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/application/emailbranding"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/application/realtimeevents"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/emailoutbox"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/notifications"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/pwa"
)

// The Delivery services the typed root binds. The root may not import
// Delivery's application packages, so the composition names the e-mail
// outbox, the notification services and the live-event publisher for it
// (ADR 0053 point 2).

type (
	EmailOutbox                   = emailoutbox.Service
	NotificationConsentStore      = notifications.ConsentStore
	Notifier                      = notifications.Notifier
	NotificationPreferenceService = notifications.PreferenceService
	PushSubscriptionService       = notifications.PushSubscriptionService
	VAPIDConfig                   = notifications.VAPIDConfig
	RealtimePublisher             = realtimeevents.Publisher
	// StaffGroupPair is one supervisor of a group, the answer the staff
	// notification recipients read from School Structure.
	StaffGroupPair = notifications.StaffGroupPair
	// SchoolContact is the reply address of tenant-bound mail, the answer
	// the mail identity reads from Organisation & Tenancy.
	SchoolContact = emailoutbox.SchoolContact
	// AbsenceNotifier and AbsenceReport carry the absences an approved
	// excused-absence request reports to staff, for Care Plan's requests
	// (#4011).
	AbsenceNotifier = notifications.AbsenceNotifier
	AbsenceReport   = notifications.AbsenceReport
	// The mail renderers of Identity & Access, which the root binds to
	// Delivery (#4006).
	MailIntent       = emailoutbox.Intent
	MailRenderer     = emailoutbox.Renderer
	MailRendererFunc = emailoutbox.RendererFunc
	// PWA usage aggregates come from the root-bound Organisation & Tenancy port.
	PWAUsageRow = pwa.UsageRow
	// The opt-in mail recipients of a notification type, which the root
	// binds to Enrollment's "Neue Anmeldung" mail (#3780, #4012).
	EmailSubscribers       = notifications.EmailSubscribers
	EmailAccounts          = notifications.EmailAccounts
	EmailRecipientResolver = notifications.EmailRecipientResolver
	// The appointment mail and notices of the calendar portal, which the
	// root binds to Delivery (#4009).
	MailEnqueueRequest             = emailoutbox.EnqueueRequest
	NotificationService            = notifications.Service
	SynchronousNotificationService = notifications.SynchronousService
	NotificationEvent              = notifications.Event
	NotificationAudience           = notifications.Audience
	NotificationAudienceScope      = notifications.AudienceScope
	// Root bindings for mail, notifications and standalone PWA usage.
	MailEnqueued                 = emailoutbox.Enqueued
	NotificationDeliveryObserver = notifications.DeliveryObserver
	StaffParentMessageNotifier   = notifications.StaffParentMessageNotifier
	StaffParentMessageReport     = notifications.StaffParentMessageReport
	PWAUsageService              = pwa.UsageService
)

// The kinds of the parent appointment notices.
const (
	ParentAppointmentPublished = notifications.ParentAppointmentPublished
	ParentAppointmentUpdated   = notifications.ParentAppointmentUpdated
	ParentAppointmentCancelled = notifications.ParentAppointmentCancelled
	ParentAppointmentReminder  = notifications.ParentAppointmentReminder
)

// NotificationEnrollmentSubmitted is the notification type of a new
// enrollment.
const NotificationEnrollmentSubmitted = notifications.TypeEnrollmentSubmitted

var (
	NewEmailOutbox                   = emailoutbox.NewService
	NewEmailTemplateRegistry         = emailoutbox.NewTemplateRegistry
	NewTenantMailIdentity            = emailoutbox.NewTenantMailIdentity
	NewNotifier                      = notifications.NewServiceWithDeliveryObserver
	NewPushSubscriptionService       = notifications.NewPushSubscriptionService
	NewPreferenceService             = notifications.NewPreferenceService
	NewEmailRecipientResolver        = notifications.NewEmailRecipientResolver
	NewStaffRecipientResolver        = notifications.NewStaffRecipientResolver
	NewStaffParentMessageNotifier    = notifications.NewStaffParentMessageNotifier
	NewTenantSSEChannel              = notifications.NewTenantSSEChannel
	NewTenantDurableWebPushChannel   = notifications.NewTenantDurableWebPushChannel
	NewTenantAbsenceNotifier         = notifications.NewTenantAbsenceNotifier
	NewTenantPreferenceService       = notifications.NewTenantPreferenceService
	NewTenantPushSubscriptionService = notifications.NewTenantPushSubscriptionService
	NewTenantPWAUsageService         = pwa.NewTenantUsageService
	// ErrRenderCancelled and ErrRenderDeferred are the answers of a renderer
	// whose mail must not go out, or not yet.
	ErrRenderCancelled = emailoutbox.ErrRenderCancelled
	ErrRenderDeferred  = emailoutbox.ErrRenderDeferred
	// ErrNoWebPushSubscribers, ErrNotificationsDisabled and
	// ErrOutsideNotificationWindow are the answers of a notice nobody
	// receives: no subscriber, notifications off, outside the tenant's window.
	ErrNoWebPushSubscribers      = notifications.ErrNoWebPushSubscribers
	ErrNotificationsDisabled     = notifications.ErrDisabled
	ErrOutsideNotificationWindow = notifications.ErrOutsideActiveWindow
	// ParentAppointmentCopy is the localized title and body of a parent
	// appointment notice.
	ParentAppointmentCopy = notifications.ParentAppointmentCopy
	// MotoLogoURL and SchoolLogoURL brand the tenant mail.
	MotoLogoURL   = emailbranding.MotoLogoURL
	SchoolLogoURL = emailbranding.SchoolLogoURL
)

// NewRequestDecisionNotifier binds consent and dispatch for request decisions.
func NewRequestDecisionNotifier(notifier NotificationService, preferences notifications.RequestDecisionPreferences) *notifications.RequestDecisionNotifier {
	return notifications.NewRequestDecisionNotifier(notifier, preferences)
}

// PWAUsageWindowDays is the reporting window the root supplies to dashboards.
const PWAUsageWindowDays = pwa.UsageWindowDays
