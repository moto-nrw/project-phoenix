package ports

import "context"

type EnqueueRequest struct {
	Kind              string
	Payload           map[string]any
	RelatedEntityType string
	RelatedEntityID   int64
	IdempotencyKey    string
}
type EmailOutbox struct {
	ID, TenantID int64
	Kind         string
	Payload      map[string]any
}

func (r *EmailOutbox) GetTenantID() int64 { return r.TenantID }

type Outbox interface {
	Enqueue(context.Context, EnqueueRequest) (*EmailOutbox, error)
	CancelPendingByRelatedEntity(context.Context, string, int64, string) (int64, error)
}
type Message struct {
	To, Subject, Template string
	Content               map[string]any
}
type Audience struct {
	TenantID                       int64
	Scope                          string
	GuardianAccountIDs, StudentIDs []int64
	// StaffAccountIDs address a staff-scoped event: the Lehrkraft of an
	// Elternsprechtag (#4801).
	StaffAccountIDs []int64
}
type Event struct {
	Type, IdempotencyKey, RelatedType string
	RelatedID                         int64
	Title, Body, DeepLink, Priority   string
	// SchoolDeepLink is the destination in moto schule.
	SchoolDeepLink string
	Audience       Audience
}
type Notifier interface {
	Notify(context.Context, Event) error
}
type SynchronousNotifier interface {
	NotifySynchronously(context.Context, Event) error
}
type Preferences interface {
	FilterNotOptedOut(context.Context, string, []int64) ([]int64, error)
	FilterOptedIn(context.Context, string, []int64) ([]int64, error)
}
