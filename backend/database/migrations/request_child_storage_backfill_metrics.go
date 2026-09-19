package migrations

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

func (run *tenantBackfillRun) monitoredBatch(ctx context.Context, batch *RequestChildStorageBatch, attempt int) (resultErr error) {
	conn, err := run.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	var pid int
	if err := conn.NewRaw(`SELECT pg_backend_pid()`).Scan(ctx, &pid); err != nil {
		return err
	}
	stop, _, err := monitorBackfillLocks(ctx, run.db, pid)
	if err != nil {
		return err
	}
	observed := false
	finish := func() error {
		wait, err := stop()
		if !observed {
			run.lockWait += wait
			observed = true
		}
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, finish()) }()
	return conn.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `SET LOCAL statement_timeout = '60s'`); err != nil {
			return err
		}
		return run.applyBatch(ctx, tx, batch, attempt, finish)
	})
}

// A failed attempt's data rolled back, but its measured contention remains
// evidence. Do not advance high-water or row counters during this flush.
func (run *tenantBackfillRun) flushFailureTelemetry(ctx context.Context) error {
	flushCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
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
