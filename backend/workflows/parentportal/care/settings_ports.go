package care

import (
	"context"
)

// SettingsService reads the school settings used by the guardian flows.
// The pickup-policy lock and fresh reads stay on the caller's transaction.
type SettingsService interface {
	ResolveBoolForTenant(context.Context, int64, string) (bool, error)
	ResolveStringForTenant(context.Context, int64, string) (string, error)
	ResolveStringForTenantInTx(context.Context, int64, string) (string, error)
	LockParentPickupChangePolicySharedForTenant(context.Context, int64) error
}

// SettingsSnapshot reads the requested keys of one school's resolved batch.
type SettingsSnapshot interface {
	Bool(string) (bool, error)
	String(string) (string, error)
}

// BatchSettingsService optionally resolves child-feature settings together.
// A failed batch is returned to the caller, not retried as individual reads.
type BatchSettingsService interface {
	SettingsService
	ResolveManyForTenant(context.Context, int64, []string) (SettingsSnapshot, error)
}
