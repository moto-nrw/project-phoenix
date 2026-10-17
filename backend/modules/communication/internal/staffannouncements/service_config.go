package announcement

import (
	"log/slog"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	deliveryPorts "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
)

// ServiceConfig is the dependency bundle. Outbox, Notifier and ParentsURL are
// optional: nil delivery dependencies leave the in-app feed working.
type ServiceConfig struct {
	Repo       usersModels.ParentAnnouncementRepository
	Settings   Settings
	Outbox     OutboxEnqueuer
	PushOutbox PushOutboxCanceller
	Notifier   deliveryPorts.NotificationSender
	// ReminderNotifier provides an outbox receipt for a claimed scheduled
	// reminder. It is intentionally separate from Notifier: publication keeps
	// its fire-and-forget notification semantics.
	ReminderNotifier deliveryPorts.DurableNotificationSender
	// Preferences gates the push by the guardian's own consent. Optional; when
	// absent no guardian is notified at all, which is the safe direction.
	Preferences deliveryPorts.NotificationPreferences
	// Deliveries records who an Elternbrief was addressed to and what happened
	// to their mail. Optional: without it the letter still publishes and mails
	// still go out, only the staff matrix stays empty.
	Deliveries DeliveryRecorder
	// Fingerprints computes the content identities an Erklärung freezes. A
	// service without it refuses to publish a declaration.
	Fingerprints Fingerprints
	ParentsURL   string
	Logger       *slog.Logger
}
