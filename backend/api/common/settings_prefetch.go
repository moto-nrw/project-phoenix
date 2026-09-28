package common

import (
	"context"

	configSvc "github.com/moto-nrw/project-phoenix/services/config"
)

// PrefetchSettings batch-resolves keys and attaches the resulting snapshot to
// the returned context, so every subsequent single-key Resolve*/HasTenantOverride
// call for those keys is served in-memory (issue #2065). Best-effort: when the
// service is not batch-capable (test mocks) or the batch read fails, the
// original context is returned and the per-key calls behave exactly as before,
// preserving each call site's error semantics.
//
// Read paths only: an attached snapshot is immutable and not evicted by
// SetValue/ResetValue or the advisory-lock helpers, so handlers that write
// settings must not prefetch the keys they write.
func PrefetchSettings(ctx context.Context, settings any, keys ...string) context.Context {
	batch, ok := settings.(interface {
		ResolveMany(context.Context, []string) (*configSvc.SettingsSnapshot, error)
	})
	if !ok {
		return ctx
	}
	snapshot, err := batch.ResolveMany(ctx, keys)
	if err != nil {
		return ctx
	}
	return configSvc.WithSettingsSnapshot(ctx, snapshot)
}

// PrefetchSettingsOrError is PrefetchSettings for a read path that must not
// continue on a failed batch read: the error is returned instead of the
// original context. A service that is not batch-capable still yields the
// original context and no error.
func PrefetchSettingsOrError(ctx context.Context, settings any, keys ...string) (context.Context, error) {
	batch, ok := settings.(interface {
		ResolveMany(context.Context, []string) (*configSvc.SettingsSnapshot, error)
	})
	if !ok {
		return ctx, nil
	}
	snapshot, err := batch.ResolveMany(ctx, keys)
	if err != nil {
		return nil, err
	}
	return configSvc.WithSettingsSnapshot(ctx, snapshot), nil
}
