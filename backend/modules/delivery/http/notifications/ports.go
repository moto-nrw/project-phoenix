package notifications

import "context"

// AudienceScope selects who receives a notification.
type AudienceScope string

// Audience describes the recipients of an event. TenantID is always required;
// the other fields depend on Scope.
type Audience struct {
	TenantID           int64
	Scope              AudienceScope
	GuardianAccountID  int64   // one recipient for ScopeGuardian
	GuardianAccountIDs []int64 // batched recipients for ScopeGuardian
	ActiveGroupID      string  // required for ScopeGroup
	StaffAccountIDs    []int64 // recipients for ScopeStaff (auth.accounts.id)

	// StudentIDs marks a guardian-scoped event as being ABOUT these children, and
	// is an authorization instruction rather than payload: the delivery
	// transaction re-reads the guardian relationships and keeps only recipients who
	// still hold parent_portal.access for at least one of them.
	//
	// Producers decide their audience in the transaction that produced the event;
	// devices are resolved later, in a transaction of their own. Between the two
	// a school can revoke a guardian's access to a child, and a push is rendered
	// on a lock screen — so the question is answered again where the sending
	// happens. It can only narrow the audience the producer chose, never widen
	// it: every listed child was already part of what the producer resolved, so
	// "at least one" cannot admit a recipient the producer excluded.
	//
	// One child for a message or a decided request, the children an appointment
	// was addressed to for the calendar producers. Leave it empty only for events
	// that are about no child at all (broadcast announcements); those are gated
	// by their own producer.
	StudentIDs []int64
}

// Event is a channel-agnostic notification. Title/Body/DeepLink are
// display-safe by contract (see the package comment); Data carries additional
// non-sensitive values (opaque IDs) for client-side routing.
type Event struct {
	Type           string // e.g. "test", later "pickup_upcoming" (#669)
	IdempotencyKey string
	RelatedType    string
	RelatedID      int64
	Audience       Audience
	Priority       string // PriorityLow/Normal/High; empty defaults to normal
	Title          string
	Body           string
	DeepLink       string // app-relative path ("/reminders"); never an absolute URL
	// SchoolDeepLink is the same destination on the school portal (#2208),
	// as an app-relative /school/... path. Empty means "this notification has
	// no place in moto schule"; a school device then opens the portal root.
	SchoolDeepLink string
	Data           map[string]string

	// Portal names the portal that asked for the event. It is only consulted
	// for TypeTest (#2208): the test notification proves the setup of the
	// portal the person is standing in, so it must not fan out into the other
	// staff portal. Empty means PortalStaff. Catalogue types ignore it — where
	// they are delivered is decided by the catalogue, not by the producer.
	Portal string
}

// Service is the entry point features use to trigger notifications.
type Service interface {
	// Notify validates the event, checks the tenant feature flag, and writes
	// durable channel intents in the caller's transaction. Enqueue failures are
	// returned. Ephemeral channels run best-effort after commit.
	Notify(ctx context.Context, event Event) error
	IsDisabled(error) bool
}

// PreferenceState is one row of the profile page: what the type is, whether the
// person agreed, and whether the school currently allows it at all.
type PreferenceState struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Group       string `json:"group"`
	Enabled     bool   `json:"enabled"`
	// Available reports whether the school allows this type right now. A type
	// can be switched on personally while unavailable — see SetForAccount.
	Available bool `json:"available"`
}

// PreferenceOverview is the payload behind the profile page's card.
type PreferenceOverview struct {
	// TenantEnabled mirrors notifications.dispatch_enabled. When false nothing
	// is delivered no matter what the person chose, and the card says so.
	TenantEnabled bool              `json:"tenant_enabled"`
	Types         []PreferenceState `json:"types"`
}

// PushSubscriptionInput is the browser's PushSubscription in wire form.
type PushSubscriptionInput struct {
	Endpoint      string
	P256dh        string
	Auth          string
	UserAgent     string
	TokenFamilyID string
}

// The HTTP adapter consumes only staff/school device and preference operations.
type PushSubscriptionService interface {
	PublicKey() (string, error)
	Subscribe(context.Context, int64, PushSubscriptionInput) error
	SubscribeSchool(context.Context, int64, PushSubscriptionInput) error
	Unsubscribe(context.Context, int64, string) error
	UnsubscribeSchool(context.Context, int64, string) error
	IsNotConfigured(error) bool
	IsInvalidSubscription(error) bool
}
type PreferenceService interface {
	GetForAccount(context.Context, int64, string) (*PreferenceOverview, error)
	SetForPortalAccount(context.Context, int64, string, string, bool) error
	DisableAllForPortalAccount(context.Context, int64, string) error
	EmailSubscribed(context.Context, int64, []string, string) (bool, error)
	SetEmailSubscribed(context.Context, int64, []string, string, bool) error
	IsUnknownType(error) bool
	IsPermissionRequired(error) bool
}

const (
	PortalStaff                  = "staff"
	PortalSchool                 = "school"
	PriorityNormal               = "normal"
	ScopeStaff     AudienceScope = "staff"
)
