package migrations

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/uptrace/bun"
)

// PresenceBackfillTelemetry aggregates committed batch evidence. Duration
// includes retries and backoff, ending just before the evidence insert/commit.
// LockWaitMS is sampled wait-only time, not statement duration. PoolWaitMS is
// time acquiring this runner's two connections, not the shared pool's counters.
type PresenceBackfillTelemetry struct {
	OldestUnmigratedSeconds float64 `json:"oldest_unmigrated_seconds"`
	LockSampleIntervalMS    int     `json:"lock_sample_interval_ms"`
	Batches                 int64   `json:"batches"`
	Retries                 int64   `json:"retries"`
	Deadlocks               int64   `json:"deadlocks"`
	BatchP95MS              float64 `json:"batch_p95_ms" bun:"batch_p95_ms"`
	BatchMaxMS              float64 `json:"batch_max_ms" bun:"batch_max_ms"`
	PoolWaitMS              float64 `json:"pool_wait_ms" bun:"pool_wait_ms"`
	LockWaitMS              float64 `json:"lock_wait_ms" bun:"lock_wait_ms"`
}

func PresenceBackfillMetrics(ctx context.Context, db *bun.DB, tenantID int64) (PresenceBackfillTelemetry, error) {
	var metrics PresenceBackfillTelemetry
	metrics.LockSampleIntervalMS = 10
	if db == nil || tenantID <= 0 {
		return metrics, fmt.Errorf("database and positive tenant ID are required")
	}
	err := db.NewRaw(`SELECT count(*) AS batches, coalesce(sum(retries), 0) AS retries,
  coalesce(sum(deadlocks), 0) AS deadlocks,
  coalesce(percentile_cont(0.95) WITHIN GROUP (ORDER BY duration_ms), 0) AS batch_p95_ms,
  coalesce(max(duration_ms), 0) AS batch_max_ms,
  coalesce(sum(pool_wait_ms), 0) AS pool_wait_ms,
  coalesce(sum(lock_wait_ms), 0) AS lock_wait_ms
  FROM active.presence_backfill_batches WHERE tenant_id = ?`, tenantID).Scan(ctx, &metrics)
	if err != nil {
		return metrics, err
	}
	err = db.NewRaw(`
 SELECT coalesce(greatest(0, extract(epoch FROM (clock_timestamp() - min(created_at)))), 0)::double precision
  FROM (
  SELECT source.created_at FROM schedule.activity_instances source
  LEFT JOIN active.activity_sessions target ON target.tenant_id = source.tenant_id AND target.schedule_instance_id = source.id
  WHERE source.tenant_id = ? AND (
   (source.status IN ('active', 'completed') AND (target.id IS NULL OR
    ROW(source.status, source.active_group_id, source.started_by, source.started_at, source.completed_at, source.completed_by, source.reopen_until, source.completion_snapshot, source.created_at, source.updated_at) IS DISTINCT FROM ROW(target.status, target.active_group_id, target.started_by, target.started_at, target.completed_at, target.completed_by, target.reopen_until, target.completion_snapshot, target.created_at, target.updated_at)))
   OR (source.status IN ('planned', 'cancelled') AND target.id IS NOT NULL))
  UNION ALL
  SELECT source.created_at FROM schedule.instance_students source
  LEFT JOIN active.activity_session_attendance target ON target.tenant_id = source.tenant_id AND target.instance_student_id = source.id
  WHERE source.tenant_id = ? AND (target.id IS NULL OR
   ROW(source.status, source.substatus, source.note, source.checked_in_at, source.checked_out_at, source.is_unplanned, source.not_scheduled, source.manual_status_at, source.student_status_day_id, source.pickup_exception_id, source.created_at, source.updated_at) IS DISTINCT FROM ROW(target.status, target.substatus, target.note, target.checked_in_at, target.checked_out_at, target.is_unplanned, target.not_scheduled, target.manual_status_at, target.student_status_day_id, target.pickup_exception_id, target.created_at, target.updated_at))
  ) outstanding`, tenantID, tenantID).Scan(ctx, &metrics.OldestUnmigratedSeconds)
	return metrics, err
}

type presenceBatchObservation struct{ poolWait, lockWait time.Duration }

// The monitor has its own connection so a blocked worker can still be sampled.
// Fail rather than silently report zero when the pool cannot supply a monitor.
func monitorPresenceLocks(ctx context.Context, db *bun.DB, pid int) (func() (time.Duration, error), time.Duration, error) {
	return monitorBackfillLocks(ctx, db, pid)
}

// monitorBackfillLocks is shared by the Presence and Staff storage copies.
func monitorBackfillLocks(ctx context.Context, db *bun.DB, pid int) (func() (time.Duration, error), time.Duration, error) {
	acquireCtx, acquireCancel := context.WithTimeout(ctx, time.Second)
	started := time.Now()
	conn, err := db.Conn(acquireCtx)
	acquireCancel()
	poolWait := time.Since(started)
	if err != nil {
		return nil, poolWait, fmt.Errorf("backfill needs a separate connection for lock sampling: %w", err)
	}
	sampleCtx, cancel := context.WithCancel(ctx)
	type sampleResult struct {
		wait time.Duration
		err  error
	}
	result := make(chan sampleResult, 1)
	go func() {
		defer func() { _ = conn.Close() }()
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		var observed sampleResult
		last := time.Now()
		for {
			select {
			case <-sampleCtx.Done():
				result <- observed
				return
			case now := <-ticker.C:
				var waiting bool
				err := conn.NewRaw(`SELECT coalesce(wait_event_type = 'Lock', false) FROM pg_stat_activity WHERE pid = ?`, pid).Scan(sampleCtx, &waiting)
				if err != nil {
					if sampleCtx.Err() == nil {
						observed.err = fmt.Errorf("sample backfill lock waits: %w", err)
					}
					result <- observed
					return
				}
				if waiting {
					observed.wait += now.Sub(last)
				}
				last = now
			}
		}
	}()
	var once sync.Once
	var final sampleResult
	stop := func() (time.Duration, error) {
		once.Do(func() { cancel(); final = <-result })
		return final.wait, final.err
	}
	return stop, poolWait, nil
}
