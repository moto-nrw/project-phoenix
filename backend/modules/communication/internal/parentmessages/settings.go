package messaging

import "context"

// Settings keeps the shared messaging gate and the owner-resolved name flag.
type Settings interface {
	ResolveBool(context.Context, string) (bool, error)
	ParentMessageStaffNameVisible(context.Context) (bool, error)
}

// EventSettings resolves detached event gates without carrying setting keys.
type EventSettings interface {
	ResolveBoolForTenant(context.Context, int64, string) (bool, error)
	ParentNotesEnabledForTenant(context.Context, int64) (bool, error)
}
