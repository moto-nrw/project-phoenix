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
	marker, ok := ctx.Value(rollbackMarkerKey{}).(*rollbackMarker)
	return ok && marker != nil && marker.requested.Load()
}
