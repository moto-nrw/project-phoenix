package application

import (
	"log/slog"
)

// ServiceConfig is the dependency bundle. Outbox, Notifier and ParentsURL are
// optional: nil delivery dependencies leave the in-app feed working.
type ServiceConfig struct {
	Repo       ParentAnnouncementRepository
	Settings   Settings
	Outbox     OutboxEnqueuer
	PushOutbox PushOutboxCanceller
	Notifier   NotificationSender
	// ReminderNotifier provides an outbox receipt for a claimed scheduled
	// reminder. It is intentionally separate from Notifier: publication keeps
	// its fire-and-forget notification semantics.
	ReminderNotifier DurableNotificationSender
	// Preferences gates the push by the guardian's own consent. Optional; when
	// absent no guardian is notified at all, which is the safe direction.
	Preferences NotificationPreferences
	// Deliveries records who an Elternbrief was addressed to and what happened
	// to their mail. Optional: without it the letter still publishes and mails
	// still go out, only the staff matrix stays empty.
	Deliveries DeliveryRecorder
	// Fingerprints computes the content identities an Erklärung freezes. A
	// service without it refuses to publish a declaration.
	Fingerprints Fingerprints
	// Signatures reads the signatures of signed answers (#4785). A service
	// without it reports every signed answer as not verifiable.
	Signatures DeclarationSignatures
	// Attachments is the file side of the attachments. A service without
	// one refuses to delete an announcement that could still own
	// attachments rather than orphaning their bytes silently.
	Attachments AttachmentPurger
	ParentsURL  string
	Logger      *slog.Logger
}
