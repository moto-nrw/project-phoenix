package devices

import "context"

// StaffPushSubscriptions manages the authenticated staff or school caller's
// devices in the current tenant. Parent capabilities remain separate.
type StaffPushSubscriptions interface {
	PublicKey() (string, error)
	Subscribe(context.Context, int64, PushSubscriptionInput) error
	SubscribeSchool(context.Context, int64, PushSubscriptionInput) error
	Unsubscribe(context.Context, int64, string) error
	UnsubscribeSchool(context.Context, int64, string) error
	IsNotConfigured(error) bool
	IsInvalidSubscription(error) bool
}

// StaffNotificationPreferences reads and records only the current account's
// staff/school decisions, including its permission-gated e-mail choices.
type StaffNotificationPreferences interface {
	GetForAccount(context.Context, int64, string) (*PreferenceOverview, error)
	SetForPortalAccount(context.Context, int64, string, string, bool) error
	DisableAllForPortalAccount(context.Context, int64, string) error
	EmailSubscribed(context.Context, int64, []string, string) (bool, error)
	SetEmailSubscribed(context.Context, int64, []string, string, bool) error
	IsUnknownType(error) bool
	IsPermissionRequired(error) bool
}
