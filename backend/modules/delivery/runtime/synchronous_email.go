package delivery

import (
	"context"
	"errors"
)

// DeliverEmail claims one committed mail with a lease and waits for transport
// acceptance. Exhausted synchronous failures are terminal, never retried by
// a background run. A pre-dispatch crash leaves the entry recoverable.
func (w *Worker) DeliverEmail(ctx context.Context, tenantID, id int64) error {
	if tenantID < 0 || id <= 0 {
		return errors.New("delivery: email intent and its scope are required")
	}
	engine, ok := w.engine.(interface {
		DispatchEmail(context.Context, int64, int64) error
	})
	if !ok {
		return errors.New("delivery: synchronous email dispatch is not configured")
	}
	return engine.DispatchEmail(ctx, tenantID, id)
}
