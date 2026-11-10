package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

// batchAttemptHooks are test seams of one storage backfill run; nil fields
// keep the production path. afterMonitorStart runs between the start of the
// lock sampler and BEGIN.
type batchAttemptHooks struct {
	afterMonitorStart func()
}

// batchStepError is the failure of one batch attempt. err is the direct
// result of the step that failed. A failed lock sampler (monitor) and a
// failed rollback are diagnostics: they are reported, never classified, so
// they cannot steer a retry or a pass restart (#6449).
type batchStepError struct {
	err         error
	monitor     bool
	monitorErr  error
	rollbackErr error
}

func (e *batchStepError) Error() string {
	msg := e.err.Error()
	if e.monitor {
		msg = "lock monitor failed: " + msg
	} else if e.monitorErr != nil {
		msg += "; lock monitor: " + e.monitorErr.Error()
	}
	if e.rollbackErr != nil {
		msg += "; rollback: " + e.rollbackErr.Error()
	}
	return msg
}

func (e *batchStepError) Unwrap() error { return e.err }

// batchCommitUnknownError marks a COMMIT whose durable outcome is unknown:
// the batch may or may not be committed. It is never replayed, and its
// pending counters are not booked again.
type batchCommitUnknownError struct{ err error }

func (e *batchCommitUnknownError) Error() string {
	return fmt.Sprintf("batch commit outcome unknown: %v", e.err)
}
func (e *batchCommitUnknownError) Unwrap() error         { return e.err }
func (e *batchCommitUnknownError) CommitOutcomeUnknown() {}

// batchRetryState is the SQLSTATE a batch loop may react to: that of the
// direct result of the failed step, including a COMMIT the server rejected.
// It is empty for a lock-monitor failure and for an unknown COMMIT outcome.
func batchRetryState(err error) string {
	step, ok := err.(*batchStepError)
	if !ok || step.monitor {
		return ""
	}
	return sqlState(step.err)
}

// batchDirectError is the error a request-child classification may inspect.
func batchDirectError(err error) error {
	if step, ok := err.(*batchStepError); ok && !step.monitor {
		return step.err
	}
	return nil
}

func isBatchCommitUnknown(err error) bool {
	_, ok := err.(*batchCommitUnknownError)
	return ok
}

// runMonitoredBatch runs one batch attempt on its own connection while a
// second connection samples its lock waits. body must call observe before it
// writes the checkpoint: observe stops the sampler once and adds its wait,
// and a sampler failure aborts the attempt. COMMIT is separated from the
// statements before it: a COMMIT the server rejected with an ERROR left
// nothing behind and keeps its SQLSTATE; any other COMMIT failure is unknown.
func runMonitoredBatch(ctx context.Context, db *bun.DB, txOptions *sql.TxOptions, hooks batchAttemptHooks,
	addLockWait func(time.Duration), body func(ctx context.Context, tx bun.Tx, observe func() error) error,
) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return &batchStepError{err: err}
	}
	defer func() { _ = conn.Close() }()
	var pid int
	if err := conn.NewRaw(`SELECT pg_backend_pid()`).Scan(ctx, &pid); err != nil {
		return &batchStepError{err: err}
	}
	stop, _, err := monitorBackfillLocks(ctx, db, pid)
	if err != nil {
		return &batchStepError{err: err, monitor: true}
	}
	monitor := &batchMonitor{stop: stop, addLockWait: addLockWait}
	defer func() { _ = monitor.observe() }()
	if hooks.afterMonitorStart != nil {
		hooks.afterMonitorStart()
	}
	tx, err := conn.BeginTx(ctx, txOptions)
	if err != nil {
		return monitor.failed(&batchStepError{err: err})
	}
	// A panic must release the transaction before the deferred conn.Close.
	defer func() { _ = tx.Rollback() }()
	if err := body(ctx, tx, monitor.observe); err != nil {
		return monitor.failed(&batchStepError{err: err, rollbackErr: rollbackDiagnostic(tx)})
	}
	if err := monitor.observe(); err != nil {
		return monitor.failed(&batchStepError{err: err, rollbackErr: rollbackDiagnostic(tx)})
	}
	if err := tx.Commit(); err != nil {
		if serverRejected(err) {
			return &batchStepError{err: err}
		}
		return &batchCommitUnknownError{err: err}
	}
	return nil
}

// batchMonitor stops the lock sampler exactly once.
type batchMonitor struct {
	stop        func() (time.Duration, error)
	addLockWait func(time.Duration)
	observed    bool
	err         error
}

func (m *batchMonitor) observe() error {
	if !m.observed {
		m.observed = true
		wait, err := m.stop()
		m.addLockWait(wait)
		if err != nil {
			m.err = fmt.Errorf("observe batch lock waits: %w", err)
		}
	}
	return m.err
}

// failed completes the error of a failed attempt with the sampler result. A
// sampler failure observed by the attempt itself is the cause of the abort.
func (m *batchMonitor) failed(step *batchStepError) error {
	alreadyObserved := m.observed
	monitorErr := m.observe()
	if alreadyObserved && monitorErr != nil && errors.Is(step.err, monitorErr) {
		step.monitor = true
		return step
	}
	step.monitorErr = monitorErr
	return step
}

func rollbackDiagnostic(tx bun.Tx) error {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		return err
	}
	return nil
}

// serverRejected reports whether COMMIT ended with a server ERROR: the server
// rolled the transaction back. FATAL errors, broken connections and context
// errors leave the outcome unknown.
func serverRejected(err error) bool {
	pgErr, ok := errors.AsType[pgdriver.Error](err)
	if !ok {
		return false
	}
	severity := pgErr.Field('V')
	if severity == "" {
		severity = pgErr.Field('S')
	}
	return severity == "ERROR"
}

// scanBackfillCopy runs the copy statement of an owner backfill, or its test
// replacement: the Contracts dropped the legacy sources it reads.
func scanBackfillCopy(ctx context.Context, tx bun.Tx, override func(context.Context, bun.Tx, ...any) error,
	query *bun.RawQuery, dest ...any,
) error {
	if override != nil {
		return override(ctx, tx, dest...)
	}
	return query.Scan(ctx, dest...)
}

// flushOwnerBatchTelemetry books the contention of failed attempts, whose data
// checkpoint rolled back. A graceful cancellation still gets a bounded flush.
func flushOwnerBatchTelemetry(ctx context.Context, db *bun.DB, backfill string, tenantID int64,
	retried, deadlocks, serialization, lockTimeouts int64, lockWait time.Duration, resultErr error,
) error {
	flushCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_, err := db.ExecContext(flushCtx, `UPDATE platform.storage_backfill_checkpoints SET
		batches_retried = batches_retried + ?, deadlocks = deadlocks + ?,
		serialization_failures = serialization_failures + ?, lock_timeouts = lock_timeouts + ?,
		lock_wait_ms = lock_wait_ms + ?, updated_at = now()
		WHERE backfill = ? AND tenant_id = ?`, retried, deadlocks, serialization,
		lockTimeouts, float64(lockWait)/float64(time.Millisecond), backfill, tenantID)
	if err != nil {
		return errors.Join(resultErr, fmt.Errorf("persist failed batch telemetry: %w", err))
	}
	return resultErr
}

// classifyRetryableSQLError reports the SQLSTATE of a deadlock (40P01),
// serialization failure (40001), or lock timeout (55P03). Callers pass the
// direct result of the failed step (batchDirectError), never a joined chain.
func classifyRetryableSQLError(err error) (string, bool) {
	var state interface{ Field(byte) string }
	if !errors.As(err, &state) {
		return "", false
	}
	switch code := state.Field('C'); code {
	case "40P01", "40001", "55P03":
		return code, true
	default:
		return code, false
	}
}
