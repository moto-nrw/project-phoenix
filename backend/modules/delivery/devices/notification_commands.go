package devices

import "context"

// NotificationAudience retains every authorization and portal selector.
type NotificationAudience struct {
	TenantID           int64
	Scope              string
	GuardianAccountID  int64
	GuardianAccountIDs []int64
	ActiveGroupID      string
	StaffAccountIDs    []int64
	StudentIDs         []int64
}

// NotificationEvent is the complete producer input to the existing router.
type NotificationEvent struct {
	Type           string
	IdempotencyKey string
	RelatedType    string
	RelatedID      int64
	Audience       NotificationAudience
	Priority       string
	Title          string
	Body           string
	DeepLink       string
	SchoolDeepLink string
	Data           map[string]string
	Portal         string
}

// NotificationCommands dispatches through the same bound Delivery router.
type NotificationCommands interface {
	NotifyEvent(context.Context, NotificationEvent) error
	NotifyEventDurably(context.Context, NotificationEvent) (int, error)
}
