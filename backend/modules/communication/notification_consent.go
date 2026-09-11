package communication

import "context"

// NotificationConsentCapability owns users.notification_preferences: one row
// per (school, account, notification type) recording that somebody agreed to
// — or explicitly declined — one kind of notification.
//
// The contract is deliberately expressed in plain values. Consumers declare
// their own port with these method signatures and never learn that the rows
// are stored at all; Delivery's PreferenceService is the only current caller
// and owns the catalogue, the portal rules, and the tenant gates on top.
//
// Two filters exist because the two channel families answer "no decision
// stored" differently:
//
//   - FilterOptedIn is consent-first. A candidate without a row is dropped, so
//     an unknown decision can never turn into a delivery. Push and in-app
//     hints use it.
//   - FilterNotOptedOut drops only the explicit declines. Channels that
//     already reached people before consent existed (e-mail) use it, so the
//     absence of a decision does not silence a school's existing mail.
//
// Both return an empty result for an empty candidate set, never "everyone".
type NotificationConsentCapability interface {
	// StoredConsent returns the account's decisions in the current school,
	// keyed by notification type. Untouched types are absent, not false.
	StoredConsent(ctx context.Context, accountID int64) (map[string]bool, error)

	// RecordConsent stores one decision, replacing any previous one for the
	// same (school, account, type).
	RecordConsent(ctx context.Context, accountID int64, notificationType string, enabled bool) error

	// DisableConsent switches the named stored decisions off. Types with no
	// row stay absent — they are already off.
	DisableConsent(ctx context.Context, accountID int64, notificationTypes []string) error

	// HasAnyOptedIn reports whether the current school holds at least one
	// enabled row for any named type. It is a scheduling gate only and never
	// the source of an audience.
	HasAnyOptedIn(ctx context.Context, notificationTypes []string) (bool, error)

	// FilterOptedIn keeps only the candidates who agreed to the type.
	FilterOptedIn(ctx context.Context, notificationType string, accountIDs []int64) ([]int64, error)

	// FilterOptedInByType answers FilterOptedIn for several types in one read.
	FilterOptedInByType(ctx context.Context, notificationTypes []string, accountIDs []int64) (map[string][]int64, error)

	// FilterNotOptedOut removes only the candidates who explicitly declined
	// the type, preserving the caller's order.
	FilterNotOptedOut(ctx context.Context, notificationType string, accountIDs []int64) ([]int64, error)
}
