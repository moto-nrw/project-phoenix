package compose

import (
	deliveryModels "github.com/moto-nrw/project-phoenix/models/delivery"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/emailbranding"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/emailoutbox"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/realtimeevents"
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
	// The appointment mail and notices of the calendar portal, which the
	// root binds to Delivery (#4009).
	MailEnqueueRequest             = emailoutbox.EnqueueRequest
	NotificationService            = notifications.Service
	SynchronousNotificationService = notifications.SynchronousService
	NotificationEvent              = notifications.Event
	NotificationAudience           = notifications.Audience
	NotificationAudienceScope      = notifications.AudienceScope
)

// The kinds of the parent appointment notices.
const (
	ParentAppointmentPublished = notifications.ParentAppointmentPublished
	ParentAppointmentUpdated   = notifications.ParentAppointmentUpdated
	ParentAppointmentCancelled = notifications.ParentAppointmentCancelled
	ParentAppointmentReminder  = notifications.ParentAppointmentReminder
)

// The portals a push subscription is bound to.
const (
	PushPortalParent = deliveryModels.PushPortalParent
	PushPortalSchool = deliveryModels.PushPortalSchool
)

var (
	NewEmailOutbox             = emailoutbox.NewService
	NewEmailTemplateRegistry   = emailoutbox.NewTemplateRegistry
	NewTenantMailIdentity      = emailoutbox.NewTenantMailIdentity
	NewNotifier                = notifications.NewServiceWithDeliveryObserver
	NewSSEChannel              = notifications.NewSSEChannel
	WithGuardianChildAccess    = notifications.WithGuardianChildAccess
	NewDurableWebPushChannel   = notifications.NewDurableWebPushChannel
	NewPushSubscriptionService = notifications.NewPushSubscriptionService
	NewPreferenceService       = notifications.NewPreferenceService
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
