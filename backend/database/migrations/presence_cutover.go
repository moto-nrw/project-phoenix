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

// presenceCutoverTimeout bounds the whole switch. The write lock it holds
// stops every timetable and attendance write in every school, so an
// unexpectedly slow final delta has to fail and be retried rather than extend
// the outage.
const presenceCutoverTimeout = 60 * time.Second

// presenceCutoverPhase marks a checkpoint whose verdict was written by the
// switch itself rather than by a backfill pass.
const presenceCutoverPhase = "cutover"

// PresenceCutoverVerification is the per-tenant verdict the switch requires:
// the two targets must reproduce the old execution and attendance columns
// row for row.
type PresenceCutoverVerification struct {
	Sessions   PresenceBackfillComparison
	Attendance PresenceBackfillComparison
	Orphans    int64
}

// Equal reports whether counts, canonical checksums and the row-wise
// comparison all agree for both targets.
func (v PresenceCutoverVerification) Equal() bool {
	return comparisonEqual(v.Sessions) && comparisonEqual(v.Attendance) && v.Orphans == 0
}

func comparisonEqual(c PresenceBackfillComparison) bool {
	return c.SourceCount == c.TargetCount && c.SourceChecksum == c.TargetChecksum && c.Mismatches == 0
}

// Describe names every failing verdict for the migration error.
func (v PresenceCutoverVerification) Describe() string {
	return fmt.Sprintf("sessions: source %d rows (%s), target %d rows (%s), %d mismatched; attendance: source %d rows (%s), target %d rows (%s), %d mismatched; %d orphans",
		v.Sessions.SourceCount, v.Sessions.SourceChecksum, v.Sessions.TargetCount, v.Sessions.TargetChecksum, v.Sessions.Mismatches,
		v.Attendance.SourceCount, v.Attendance.SourceChecksum, v.Attendance.TargetCount, v.Attendance.TargetChecksum, v.Attendance.Mismatches,
		v.Orphans)
}

// finalizePresenceStorage applies the last backfill delta, proves the two
// targets reproduce the old columns for every school, and installs the
// compatibility shape, all inside one transaction that holds the write locks.
//
// switchSchema is the schema change itself. It is a parameter so the cutover
// test can drive the same lock, delta and verification against a failing or
// partial switch and prove the transaction rolls all of it back together.
func finalizePresenceStorage(ctx context.Context, db *bun.DB, switchSchema func(context.Context, bun.Tx) error) error {
	if db == nil {
		return errors.New("presence cutover: database is required")
	}
	// Running twice must not install the triggers twice; presenceCutoverUp
	// resumes as a no-op when they are in place; this helper is only the
	// switch itself and still refuses.
	installed, err := presenceCompatibilityInstalled(ctx, db)
	if err != nil {
		return err
	}
	if installed {
		return errors.New("presence cutover: the compatibility triggers are already installed")
	}
	ctx, cancel := context.WithTimeout(ctx, presenceCutoverTimeout)
	defer cancel()
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// The table locks also fence a running backfill batch: it holds the
		// checkpoint row lock only after it opened its transaction on the
		// source tables, which now waits behind this lock.
		if _, err := tx.ExecContext(ctx, `
			SET LOCAL lock_timeout = '5s';
			SET LOCAL statement_timeout = '60s';
			SET LOCAL TIME ZONE 'UTC';
			LOCK TABLE schedule.activity_instances, schedule.instance_students,
				active.activity_sessions, active.activity_session_attendance
				IN ACCESS EXCLUSIVE MODE`); err != nil {
			return fmt.Errorf("presence cutover: lock timetable and presence storage: %w", err)
		}
		var tenantIDs []int64
		if err := tx.NewRaw(`SELECT id FROM platform.schools ORDER BY id`).Scan(ctx, &tenantIDs); err != nil {
			return fmt.Errorf("presence cutover: list schools: %w", err)
		}
		for _, tenantID := range tenantIDs {
			if err := reconcilePresenceFinalDelta(ctx, tx, tenantID); err != nil {
				return err
			}
		}
		return switchSchema(ctx, tx)
	})
}

// presenceCompatibilityInstalled reports whether the rollback-only mirror is in
// place: the switch has run when the session mirror trigger exists.
func presenceCompatibilityInstalled(ctx context.Context, db bun.IDB) (bool, error) {
	var installed bool
	if err := db.NewRaw(`SELECT EXISTS (
		SELECT 1 FROM pg_trigger WHERE tgname = 'activity_sessions_mirror'
		  AND tgrelid = 'active.activity_sessions'::regclass AND NOT tgisinternal)`).Scan(ctx, &installed); err != nil {
		return false, fmt.Errorf("presence cutover: inspect compatibility triggers: %w", err)
	}
	return installed, nil
}

// requirePresenceStorageBeforeCutover refuses the backfill and its restart once
// the owner tables are authoritative: a restart would erase Presence data and
// a batch would copy the mirror back onto itself.
func requirePresenceStorageBeforeCutover(ctx context.Context, db *bun.DB) error {
	installed, err := presenceCompatibilityInstalled(ctx, db)
	if err != nil {
		return err
	}
	if installed {
		return errors.New("presence backfill: the storage is already cut over (#2762); active.activity_sessions and active.activity_session_attendance are authoritative")
	}
	return nil
}

// reconcilePresenceFinalDelta copies everything the resumable backfill has not
// seen yet for one school and then holds the two shapes against each other.
// Unlike a backfill pass it is not keyset-batched: the write lock is already
// held and the delta is the small remainder.
func reconcilePresenceFinalDelta(ctx context.Context, tx bun.Tx, tenantID int64) error {
	report, err := readPresenceCheckpoint(ctx, tx, tenantID, true)
	if errors.Is(err, sql.ErrNoRows) {
		report = PresenceBackfillReport{TenantID: tenantID, Phase: "sessions", Pass: 1}
		err = nil
	}
	if err != nil {
		return fmt.Errorf("presence cutover: tenant %d checkpoint: %w", tenantID, err)
	}
	if !report.Complete {
		var hasRows bool
		if err := tx.NewRaw(`SELECT EXISTS (SELECT 1 FROM schedule.activity_instances WHERE tenant_id = ?0)
			OR EXISTS (SELECT 1 FROM schedule.instance_students WHERE tenant_id = ?0)`, tenantID).Scan(ctx, &hasRows); err != nil {
			return fmt.Errorf("presence cutover: tenant %d source rows: %w", tenantID, err)
		}
		if hasRows {
			return fmt.Errorf("presence cutover: tenant %d requires a completed backfill pass; run `migrate presence-backfill run --tenant-id %d` first", tenantID, tenantID)
		}
	}
	copiedSessions, removed, err := copyPresenceSessions(ctx, tx, tenantID, nil)
	if err != nil {
		return fmt.Errorf("presence cutover: tenant %d final session delta: %w", tenantID, err)
	}
	copiedAttendance, err := copyPresenceAttendance(ctx, tx, tenantID, nil)
	if err != nil {
		return fmt.Errorf("presence cutover: tenant %d final attendance delta: %w", tenantID, err)
	}
	verification, err := verifyPresenceCutover(ctx, tx, tenantID)
	if err != nil {
		return err
	}
	if !verification.Equal() {
		return fmt.Errorf("presence cutover: tenant %d is not reproducible: %s", tenantID, verification.Describe())
	}
	report.Sessions, report.Attendance, report.Orphans = verification.Sessions, verification.Attendance, verification.Orphans
	report.Mismatches = 0
	report.RowsCopied += copiedSessions + copiedAttendance
	report.RowsRemoved += removed
	report.BatchesCompleted++
	return persistPresenceCutoverEvidence(ctx, tx, report)
}

// verifyPresenceCutover reuses the backfill's own projections and checksums so
// the switch proves exactly what the backfill promised, in the same
// repeatable snapshot as the delta it just applied.
func verifyPresenceCutover(ctx context.Context, tx bun.Tx, tenantID int64) (PresenceCutoverVerification, error) {
	report := PresenceBackfillReport{TenantID: tenantID, Phase: "verify"}
	if err := verifyPresenceBackfill(ctx, tx, &report); err != nil {
		return PresenceCutoverVerification{}, fmt.Errorf("presence cutover: tenant %d verify: %w", tenantID, err)
	}
	return PresenceCutoverVerification{Sessions: report.Sessions, Attendance: report.Attendance, Orphans: report.Orphans}, nil
}

// persistPresenceCutoverEvidence leaves the switch's own verdict in the
// checkpoint the backfill wrote. The row is what an operator reads during the
// rollback window, so the final delta must be visible in it and not only in
// the migration log.
func persistPresenceCutoverEvidence(ctx context.Context, tx bun.Tx, report PresenceBackfillReport) error {
	var verifiedAt time.Time
	if err := tx.NewRaw(`SELECT clock_timestamp(), txid_current_snapshot()::text`).Scan(ctx, &verifiedAt, &report.FinalDeltaSnapshot); err != nil {
		return fmt.Errorf("presence cutover: tenant %d snapshot: %w", report.TenantID, err)
	}
	report.VerifiedAt = &verifiedAt
	report.Complete = true
	report.Phase = presenceCutoverPhase
	report.SessionHighWater, report.AttendanceHighWater = 0, 0
	state, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("presence cutover: tenant %d encode checkpoint: %w", report.TenantID, err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO active.presence_backfill_checkpoints (tenant_id, state)
		VALUES (?, ?::jsonb)
		ON CONFLICT (tenant_id) DO UPDATE SET state = EXCLUDED.state, updated_at = clock_timestamp()`,
		report.TenantID, string(state)); err != nil {
		return fmt.Errorf("presence cutover: tenant %d persist verification: %w", report.TenantID, err)
	}
	return nil
}

// presenceCutoverPrecondition answers, before the deployment stops the
// application, whether a school holds timetable rows without a completed
// backfill pass. Counts and checksums are mechanical and are closed by the
// switch's own final delta; a missing pass is what only an operator can fix.
func presenceCutoverPrecondition(ctx context.Context, db *bun.DB) error {
	if db == nil {
		return errors.New("presence cutover preflight: database is required")
	}
	installed, err := presenceCompatibilityInstalled(ctx, db)
	if err != nil {
		return err
	}
	if installed {
		return nil
	}
	var missing []int64
	if err := db.NewRaw(`SELECT s.id FROM platform.schools AS s
		WHERE (EXISTS (SELECT 1 FROM schedule.activity_instances i WHERE i.tenant_id = s.id)
		    OR EXISTS (SELECT 1 FROM schedule.instance_students p WHERE p.tenant_id = s.id))
		  AND NOT COALESCE((SELECT (c.state->>'complete')::boolean FROM active.presence_backfill_checkpoints c WHERE c.tenant_id = s.id), false)
		ORDER BY s.id`).Scan(ctx, &missing); err != nil {
		return fmt.Errorf("presence cutover preflight: %w", err)
	}
	if len(missing) > 0 {
		return fmt.Errorf("presence cutover preflight: schools %v have timetable rows but no completed presence backfill pass; run `migrate presence-backfill run --all-tenants` before the release", missing)
	}
	return nil
}
