package migrations

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

// PresenceBackfillComparison compares canonical, ID-ordered field payloads in
// one repeatable-read snapshot. Target surrogate IDs are not source identities.
type PresenceBackfillComparison struct {
	SourceCount             int64   `json:"source_count"`
	TargetCount             int64   `json:"target_count"`
	SourceChecksum          string  `json:"source_checksum"`
	TargetChecksum          string  `json:"target_checksum"`
	Mismatches              int64   `json:"mismatches"`
	OldestUnmigratedSeconds float64 `json:"oldest_unmigrated_seconds"`
}

// PresenceBackfillReport is persisted atomically with each batch. Complete is
// evidence at VerifiedAt, not permission to switch callers: old writers remain
// authoritative and Cutover must run another final delta after fencing them.
type PresenceBackfillReport struct {
	TenantID                  int64                      `json:"tenant_id"`
	Phase                     string                     `json:"phase"`
	Pass                      int64                      `json:"pass"`
	SessionHighWater          int64                      `json:"session_high_water"`
	AttendanceHighWater       int64                      `json:"attendance_high_water"`
	RowsScanned               int64                      `json:"rows_scanned"`
	RowsCopied                int64                      `json:"rows_copied"`
	RowsSkipped               int64                      `json:"rows_skipped"`
	RowsRemoved               int64                      `json:"rows_removed"`
	BatchesCompleted          int64                      `json:"batches_completed"`
	Mismatches                int64                      `json:"mismatches"`
	Orphans                   int64                      `json:"orphans"`
	Complete                  bool                       `json:"complete"`
	VerifiedAt                *time.Time                 `json:"verified_at,omitempty"`
	FinalDeltaSnapshot        string                     `json:"final_delta_snapshot,omitempty"`
	Sessions                  PresenceBackfillComparison `json:"sessions"`
	Attendance                PresenceBackfillComparison `json:"attendance"`
	PlannedOccurrences        int64                      `json:"planned_occurrences"`
	PlannedAssignments        int64                      `json:"planned_assignments"`
	PlannedOccurrenceChecksum string                     `json:"planned_occurrence_checksum"`
	PlannedAssignmentChecksum string                     `json:"planned_assignment_checksum"`
}

// PresenceBackfillStatus reads the durable checkpoint without running a delta.
func PresenceBackfillStatus(ctx context.Context, db *bun.DB, tenantID int64) (PresenceBackfillReport, error) {
	if db == nil || tenantID <= 0 {
		return PresenceBackfillReport{}, fmt.Errorf("database and positive tenant ID are required")
	}
	report, err := readPresenceCheckpoint(ctx, db, tenantID, false)
	if errors.Is(err, sql.ErrNoRows) {
		return PresenceBackfillReport{TenantID: tenantID, Phase: "not_started"}, nil
	}
	return report, err
}

func readPresenceCheckpoint(ctx context.Context, db bun.IDB, tenantID int64, lock bool) (PresenceBackfillReport, error) {
	var data []byte
	var err error
	if lock {
		err = db.NewRaw(`SELECT state FROM active.presence_backfill_checkpoints WHERE tenant_id = ? FOR UPDATE`, tenantID).Scan(ctx, &data)
	} else {
		err = db.NewRaw(`SELECT state FROM active.presence_backfill_checkpoints WHERE tenant_id = ?`, tenantID).Scan(ctx, &data)
	}
	if err != nil {
		return PresenceBackfillReport{}, err
	}
	var report PresenceBackfillReport
	err = json.Unmarshal(data, &report)
	return report, err
}

// PresenceBackfillBatch commits at most batchSize source rows for one tenant.
// Calling it again resumes from storage, including after process termination.
// A completed checkpoint starts a fresh full pass to catch old-row changes;
// no updated_at cursor or dual write can miss edits to already-scanned IDs.
func PresenceBackfillBatch(ctx context.Context, db *bun.DB, tenantID int64, batchSize int) (PresenceBackfillReport, error) {
	if db == nil || tenantID <= 0 || batchSize < 1 || batchSize > 10000 {
		return PresenceBackfillReport{}, fmt.Errorf("database, positive tenant ID and batch size between 1 and 10000 are required")
	}
	started := time.Now()
	var retries, deadlocks int
	var observation presenceBatchObservation
	for {
		report, err := presenceBackfillAttempt(ctx, db, tenantID, batchSize, started, retries, deadlocks, &observation)
		if err == nil {
			return report, nil
		}
		var pgErr interface {
			error
			Field(byte) string
		}
		var code string
		if errors.As(err, &pgErr) {
			code = pgErr.Field('C')
		}
		if code == "40P01" {
			deadlocks++
		}
		if (code != "40001" && code != "40P01" && code != "55P03") || retries == 5 {
			return PresenceBackfillReport{}, fmt.Errorf("presence backfill tenant %d (retries=%d deadlocks=%d): %w", tenantID, retries, deadlocks, err)
		}
		retries++
		timer := time.NewTimer(time.Duration(25*(1<<retries)) * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return PresenceBackfillReport{}, ctx.Err()
		case <-timer.C:
		}
	}
}

func presenceBackfillAttempt(ctx context.Context, db *bun.DB, tenantID int64, batchSize int, started time.Time, retries, deadlocks int, observation *presenceBatchObservation) (PresenceBackfillReport, error) {
	initial := PresenceBackfillReport{TenantID: tenantID, Phase: "sessions", Pass: 1}
	var report PresenceBackfillReport
	poolStarted := time.Now()
	conn, err := db.Conn(ctx)
	if err != nil {
		return report, err
	}
	observation.poolWait += time.Since(poolStarted)
	defer func() { _ = conn.Close() }()
	var pid int
	if err := conn.NewRaw(`SELECT pg_backend_pid()`).Scan(ctx, &pid); err != nil {
		return report, err
	}
	stopMonitor, monitorPoolWait, err := monitorPresenceLocks(ctx, db, pid)
	observation.poolWait += monitorPoolWait
	if err != nil {
		return report, err
	}
	defer func() { _, _ = stopMonitor() }()
	err = conn.RunInTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `SET LOCAL lock_timeout = '5s'; SET LOCAL statement_timeout = '60s'; SET LOCAL TIME ZONE 'UTC'`); err != nil {
			return err
		}
		data, err := json.Marshal(initial)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO active.presence_backfill_checkpoints (tenant_id, state) VALUES (?, ?::jsonb) ON CONFLICT (tenant_id) DO NOTHING`, tenantID, string(data)); err != nil {
			return err
		}
		report, err = readPresenceCheckpoint(ctx, tx, tenantID, true)
		if err != nil {
			return err
		}
		if report.Complete {
			report.Complete = false
			report.Phase = "sessions"
			report.SessionHighWater, report.AttendanceHighWater = 0, 0
			report.Pass++
			report.VerifiedAt = nil
			report.FinalDeltaSnapshot = ""
		}
		if err := advancePresenceBackfill(ctx, tx, &report, batchSize); err != nil {
			return err
		}
		lockWait, err := stopMonitor()
		if err != nil {
			return err
		}
		report.BatchesCompleted++
		data, err = json.Marshal(report)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE active.presence_backfill_checkpoints SET state = ?::jsonb, updated_at = clock_timestamp() WHERE tenant_id = ?`, string(data), tenantID); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO active.presence_backfill_batches
   (tenant_id, batch_number, duration_ms, pool_wait_ms, lock_wait_ms, retries, deadlocks)
   VALUES (?, ?, ?, ?, ?, ?, ?)`, tenantID, report.BatchesCompleted, float64(time.Since(started))/float64(time.Millisecond), float64(observation.poolWait)/float64(time.Millisecond), float64(observation.lockWait+lockWait)/float64(time.Millisecond), retries, deadlocks)
		return err
	})
	lockWait, monitorErr := stopMonitor()
	observation.lockWait += lockWait
	if err == nil {
		err = monitorErr
	}
	return report, err
}

func advancePresenceBackfill(ctx context.Context, tx bun.Tx, report *PresenceBackfillReport, batchSize int) error {
	if report.Phase == "verify" {
		return verifyPresenceBackfill(ctx, tx, report)
	}
	var ids []int64
	var err error
	switch report.Phase {
	case "sessions":
		err = tx.NewRaw(`SELECT id FROM schedule.activity_instances WHERE tenant_id = ? AND id > ? ORDER BY id LIMIT ?`, report.TenantID, report.SessionHighWater, batchSize).Scan(ctx, &ids)
	case "attendance":
		err = tx.NewRaw(`SELECT id FROM schedule.instance_students WHERE tenant_id = ? AND id > ? ORDER BY id LIMIT ?`, report.TenantID, report.AttendanceHighWater, batchSize).Scan(ctx, &ids)
	default:
		return fmt.Errorf("invalid Presence checkpoint phase %q", report.Phase)
	}
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		if report.Phase == "sessions" {
			report.Phase = "attendance"
		} else {
			report.Phase = "verify"
		}
		return nil
	}
	var copied, removed int64
	if report.Phase == "sessions" {
		copied, removed, err = copyPresenceSessions(ctx, tx, report.TenantID, ids)
		report.SessionHighWater = ids[len(ids)-1]
	} else {
		copied, err = copyPresenceAttendance(ctx, tx, report.TenantID, ids)
		report.AttendanceHighWater = ids[len(ids)-1]
	}
	if err != nil {
		return err
	}
	report.RowsScanned += int64(len(ids))
	report.RowsCopied += copied
	report.RowsRemoved += removed
	report.RowsSkipped += int64(len(ids)) - copied
	return nil
}
