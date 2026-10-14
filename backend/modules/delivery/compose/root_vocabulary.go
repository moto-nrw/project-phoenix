package compose

import (
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
)
