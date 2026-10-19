package postgres

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication/internal/ports"
)

// NewMessageObservation binds query attribution and completion to a store operation.
func NewMessageObservation(observe func(ports.Observation)) func(context.Context, string) (context.Context, func(error)) {
	return func(ctx context.Context, operation string) (context.Context, func(error)) {
		started := time.Now()
		runCtx, queryStats := WithMessageQueryStats(ctx)
		return runCtx, func(err error) {
			if observe != nil {
				stats := queryStats.Snapshot()
				if err != nil {
					stats.Rows = 0
				}
				observe(ports.Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
			}
		}
	}
}
