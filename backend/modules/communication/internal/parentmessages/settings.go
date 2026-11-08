package messaging

import "context"

// Settings keeps the shared messaging gate and the owner-resolved name flag.
type Settings interface {
	ResolveBool(context.Context, string) (bool, error)
	ParentMessageStaffNameVisible(context.Context) (bool, error)
	// ParentMessageLeadershipEnabled resolves whether parents may start a
	// leadership conversation (#5200).
	ParentMessageLeadershipEnabled(context.Context) (bool, error)
	// CareFacilityTerm resolves the school's term for its care, which the
	// guardian's new-message mail names (#6082).
	CareFacilityTerm(context.Context) (string, error)
}

// EventSettings resolves detached event gates without carrying setting keys.
type EventSettings interface {
	ResolveBoolForTenant(context.Context, int64, string) (bool, error)
	ParentNotesEnabledForTenant(context.Context, int64) (bool, error)
}
