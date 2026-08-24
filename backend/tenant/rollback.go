package tenant

import (
	"context"
	"sync/atomic"
)

type rollbackMarkerKey struct{}

type rollbackMarker struct {
	requested atomic.Bool
}

// WithRollbackMarker attaches the marker used by MarkRollback and
// RollbackRequested. Re-wrapping a context preserves the existing marker.
func WithRollbackMarker(ctx context.Context) context.Context {
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

// RollbackRequested reports whether MarkRollback was called on this context.
// Read-only: it lets a handler test assert that a non-5xx error path asked for
// a rollback without standing up the whole transaction middleware.
func RollbackRequested(ctx context.Context) bool {
	marker, ok := ctx.Value(rollbackMarkerKey{}).(*rollbackMarker)
	return ok && marker != nil && marker.requested.Load()
}
