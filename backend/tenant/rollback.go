package tenant

import (
	"context"
	"sync/atomic"
)

type rollbackMarkerKey struct{}

type rollbackMarker struct {
	requested atomic.Bool
}

func withRollbackMarker(ctx context.Context) context.Context {
	if _, ok := ctx.Value(rollbackMarkerKey{}).(*rollbackMarker); ok {
		return ctx
	}
	return context.WithValue(ctx, rollbackMarkerKey{}, &rollbackMarker{})
}

// MarkRollback requests rollback of the surrounding tenant transaction even
// when the handler has already rendered a non-5xx response.
func MarkRollback(ctx context.Context) {
	if marker, ok := ctx.Value(rollbackMarkerKey{}).(*rollbackMarker); ok && marker != nil {
		marker.requested.Store(true)
	}
}

func rollbackRequested(ctx context.Context) bool {
	return RollbackRequested(ctx)
}

// RollbackRequested reports whether MarkRollback was called on this context.
// Read-only: it lets a handler test assert that a non-5xx error path asked for
// a rollback without standing up the whole transaction middleware.
func RollbackRequested(ctx context.Context) bool {
	marker, ok := ctx.Value(rollbackMarkerKey{}).(*rollbackMarker)
	return ok && marker != nil && marker.requested.Load()
}

// WithRollbackMarker attaches a rollback marker to ctx, so MarkRollback and
// RollbackRequested work outside the tenant transaction middleware (tests).
func WithRollbackMarker(ctx context.Context) context.Context {
	return withRollbackMarker(ctx)
}
