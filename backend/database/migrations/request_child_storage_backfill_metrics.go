package migrations

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

// requestChildSourceCounts are the row counts of the source statements of one
// request-child batch.
type requestChildSourceCounts struct {
	scannedSource, deletedBookings, insertedBookings, deletedSelections, insertedSelections int64
}

func (run *tenantBackfillRun) monitoredBatch(ctx context.Context, batch *RequestChildStorageBatch, attempt int) error {
	return runMonitoredBatch(ctx, run.db, nil, run.options.attemptHooks,
		func(wait time.Duration) { run.lockWait += wait },
		func(ctx context.Context, tx bun.Tx, observe func() error) error {
			if _, err := tx.ExecContext(ctx, `SET LOCAL statement_timeout = '60s'`); err != nil {
				return err
			}
			return run.applyBatch(ctx, tx, batch, attempt, observe)
		})
}

// A failed attempt's data rolled back, but its measured contention remains
// evidence. Do not advance high-water or row counters during this flush.
// After a COMMIT with unknown outcome the batch may already hold the pending
// counters: bookCounters is false and only acceptance is withdrawn (#6449).
func (run *tenantBackfillRun) flushFailureTelemetry(ctx context.Context, bookCounters bool) error {
	flushCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if !bookCounters {
		_, err := run.db.NewRaw(`UPDATE enrollment.request_child_storage_backfill_checkpoints SET
			complete = FALSE, stable = FALSE, updated_at = now() WHERE tenant_id = ?`, run.tenantID).Exec(flushCtx)
		if err != nil {
			return fmt.Errorf("withdraw request-child acceptance after unknown commit: %w", err)
		}
		return nil
	}
	_, err := run.db.NewRaw(`UPDATE enrollment.request_child_storage_backfill_checkpoints SET
		batches_retried = batches_retried + ?, deadlocks = deadlocks + ?,
		serialization_failures = serialization_failures + ?, lock_timeouts = lock_timeouts + ?,
		lock_wait_ms = lock_wait_ms + ?, complete = FALSE, stable = FALSE, updated_at = now()
		WHERE tenant_id = ?`, run.retried, run.deadlocks, run.serializationFailures, run.lockTimeouts,
		float64(run.lockWait)/float64(time.Millisecond), run.tenantID).Exec(flushCtx)
	if err != nil {
		return fmt.Errorf("persist failed request-child batch telemetry: %w", err)
	}
	run.checkpoint.BatchesRetried += run.retried
	run.checkpoint.Deadlocks += run.deadlocks
	run.checkpoint.SerializationFailures += run.serializationFailures
	run.checkpoint.LockTimeouts += run.lockTimeouts
	run.checkpoint.LockWaitMs += float64(run.lockWait) / float64(time.Millisecond)
	run.retried, run.deadlocks, run.serializationFailures, run.lockTimeouts, run.lockWait = 0, 0, 0, 0, 0
	return nil
}
