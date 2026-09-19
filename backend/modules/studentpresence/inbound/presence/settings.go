package presence

import "context"

// Settings is the read-only settings contract used by active HTTP handlers.
type Settings interface {
	ResolveBool(context.Context, string) (bool, error)
	ResolveString(context.Context, string) (string, error)
	HasTenantOverride(context.Context, string) (bool, error)
}
