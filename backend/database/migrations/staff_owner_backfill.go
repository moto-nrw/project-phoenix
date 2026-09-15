package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

// StaffOwnerBackfillName keys the staff Membership/Workforce backfill in the
// shared checkpoint table.
const StaffOwnerBackfillName = "staff-owner"

const (
	defaultStaffOwnerBatchSize   = 500
	defaultStaffOwnerMaxPasses   = 5
	defaultStaffOwnerMaxAttempts = 5
	staffOwnerRetryBackoff       = 50 * time.Millisecond
)

// StaffOwnerBackfillOptions tunes RunStaffOwnerBackfill. Zero values select
// the defaults. The unexported seams exist for tests that interrupt the run
// at batch boundaries or inject transient database failures.
type StaffOwnerBackfillOptions struct {
	// BatchSize bounds the number of users.staff rows read per transaction.
	BatchSize int
	// MaxPasses bounds the re-read passes per tenant in one run. A tenant is
	// stable when a full pass changes nothing and verification matches.
	MaxPasses int
	// MaxAttempts bounds retries of one batch after deadlock, serialization,
	// lock-timeout, or unique-conflict restarts.
	MaxAttempts int
	Logger      *slog.Logger

	// afterBatch runs after each committed batch. Returning an error stops the
	// run; the committed checkpoint stays behind for the next run.
	afterBatch func(tenantID int64, batch staffOwnerBatch) error
	// injectFault runs inside the batch transaction before commit.
	injectFault func(ctx context.Context, tx bun.Tx, tenantID int64, attempt int) error
	// afterSourceVerification permits a deterministic concurrent source edit.
	afterSourceVerification func(context.Context, bun.Tx) error
}

func (o StaffOwnerBackfillOptions) withDefaults() StaffOwnerBackfillOptions {
	if o.BatchSize <= 0 {
		o.BatchSize = defaultStaffOwnerBatchSize
	}
	if o.MaxPasses <= 0 {
		o.MaxPasses = defaultStaffOwnerMaxPasses
	}
	if o.MaxAttempts <= 0 {
		o.MaxAttempts = defaultStaffOwnerMaxAttempts
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	return o
}

// StaffOwnerBackfillCheckpoint is the persisted per-tenant state and runtime
// evidence of the backfill. Counters are cumulative across runs and passes;
// Pass, HighWaterID and PassWrites describe the pass in progress.
type StaffOwnerBackfillCheckpoint struct {
	bun.BaseModel `bun:"table:platform.storage_backfill_checkpoints,alias:cp"`

	Backfill    string `bun:"backfill" json:"backfill"`
	TenantID    int64  `bun:"tenant_id" json:"tenant_id"`
	Pass        int    `bun:"pass" json:"pass"`
	HighWaterID int64  `bun:"high_water_id" json:"high_water_id"`
	PassWrites  int64  `bun:"pass_writes" json:"pass_writes"`
	// PassCompleted marks a pass that reached the end of the table and was
	// verified; the next run starts a fresh pass instead of resuming it.
	PassCompleted bool `bun:"pass_completed" json:"pass_completed"`
	Stable        bool `bun:"stable" json:"stable"`

	RowsScanned  int64 `bun:"rows_scanned" json:"rows_scanned"`
	RowsCopied   int64 `bun:"rows_copied" json:"rows_copied"`
	RowsSkipped  int64 `bun:"rows_skipped" json:"rows_skipped"`
	RowsRejected int64 `bun:"rows_rejected" json:"rows_rejected"`
	RowsRemoved  int64 `bun:"rows_removed" json:"rows_removed"`

	BatchesCompleted      int64 `bun:"batches_completed" json:"batches_completed"`
	BatchesRetried        int64 `bun:"batches_retried" json:"batches_retried"`
	Deadlocks             int64 `bun:"deadlocks" json:"deadlocks"`
	SerializationFailures int64 `bun:"serialization_failures" json:"serialization_failures"`
	LockTimeouts          int64 `bun:"lock_timeouts" json:"lock_timeouts"`

	SourceCount        int64      `bun:"source_count" json:"source_count"`
	TargetCount        int64      `bun:"target_count" json:"target_count"`
	SourceChecksum     string     `bun:"source_checksum" json:"source_checksum"`
	TargetChecksum     string     `bun:"target_checksum" json:"target_checksum"`
	MismatchCount      int64      `bun:"mismatch_count" json:"mismatch_count"`
	OldestUnmigratedAt *time.Time `bun:"oldest_unmigrated_at" json:"oldest_unmigrated_at,omitempty"`

	BatchP95Ms           int64   `bun:"batch_p95_ms" json:"batch_p95_ms"`
	BatchMaxMs           int64   `bun:"batch_max_ms" json:"batch_max_ms"`
	PoolWaitMs           int64   `bun:"pool_wait_ms" json:"pool_wait_ms"`
	LockWaitMs           float64 `bun:"lock_wait_ms" json:"lock_wait_ms"`
	VerificationSnapshot string  `bun:"verification_snapshot" json:"verification_snapshot"`

	VerifiedAt *time.Time `bun:"verified_at" json:"verified_at,omitempty"`
	StableAt   *time.Time `bun:"stable_at" json:"stable_at,omitempty"`
	UpdatedAt  time.Time  `bun:"updated_at" json:"updated_at"`
}

// Verified reports whether the last verification found equal counts and
// checksums and no mismatched or orphaned rows.
func (c StaffOwnerBackfillCheckpoint) Verified() bool {
	return c.VerifiedAt != nil && c.MismatchCount == 0 &&
		c.SourceCount == c.TargetCount && c.SourceChecksum == c.TargetChecksum
}

// OldestUnmigratedAge is the age of the oldest source change not yet
// reflected in the targets at the last verification, or zero.
func (c StaffOwnerBackfillCheckpoint) OldestUnmigratedAge(now time.Time) time.Duration {
	if c.OldestUnmigratedAt == nil {
		return 0
	}
	return now.Sub(*c.OldestUnmigratedAt)
}

// StaffOwnerBackfillReport is the per-tenant state after a run or status read.
type StaffOwnerBackfillReport struct {
	Tenants []StaffOwnerBackfillCheckpoint `json:"tenants"`
	// MissingTenants lists schools without any checkpoint: they have never
	// been visited, for example after a reset or when a school was created
	// after the last run.
	MissingTenants []int64 `json:"missing_tenants"`
}

// Stable reports whether every school has a verified, stable checkpoint. This
// is the Cutover precondition; it says nothing about writes after StableAt.
func (r *StaffOwnerBackfillReport) Stable() bool {
	return r != nil && len(r.Unstable()) == 0
}

// Unstable returns the schools that still block Cutover: unvisited ones and
// those whose last pass changed rows or failed verification.
func (r *StaffOwnerBackfillReport) Unstable() []int64 {
	var ids []int64
	if r == nil {
		return ids
	}
	ids = append(ids, r.MissingTenants...)
	for _, tenant := range r.Tenants {
		if !tenant.Stable || !tenant.Verified() {
			ids = append(ids, tenant.TenantID)
		}
	}
	slices.Sort(ids)
	return ids
}

type staffOwnerBatch struct {
	Scanned  int64
	LastID   int64
	Rejected int64
	Copied   int64
	Skipped  int64
	Attempts int
	Duration time.Duration
}

// staffOwnerEligibleBatch selects one keyset batch of source rows. Rows whose
// work-time model belongs to another tenant are excluded: the superuser
// connection bypasses the tenant policy that rejects them for the
// application role, so the backfill must not launder such references into
// the target. They count as rejected and keep the tenant unverified.
const staffOwnerEligibleBatch = `
	batch AS (
		SELECT s.id, s.tenant_id, s.person_id, s.created_at, s.updated_at, s.deleted_at,
		       s.staff_notes, s.employment_type, s.work_time_model_id, s.personnel_number,
		       s.rotation_anchor_date, s.birthday_display_opt_out
		FROM users.staff AS s
		WHERE s.tenant_id = ? AND s.id > ?
		ORDER BY s.id
		LIMIT ?
	),
	eligible AS (
		SELECT b.*
		FROM batch AS b
		LEFT JOIN config.work_time_models AS w ON w.id = b.work_time_model_id
		WHERE b.work_time_model_id IS NULL OR w.tenant_id = b.tenant_id
	)`

// staffOwnerCopyBatch copies one batch into both targets and returns the
// batch statistics. Membership identity is preserved (membership id = staff
// id). Both upserts only touch rows whose content differs, so reruns of a
// completed batch are no-ops and count as skipped.
const staffOwnerCopyBatch = `
	WITH ` + staffOwnerEligibleBatch + `,
	memberships AS (
		INSERT INTO users.staff_school_memberships AS m
			(id, tenant_id, person_id, created_at, updated_at, deleted_at)
		SELECT id, tenant_id, person_id, created_at, updated_at, deleted_at FROM eligible
		-- The active-person unique index is immediate: retire memberships
		-- before inserting their replacements, even when the new ID is lower.
		ORDER BY (deleted_at IS NULL), id
		ON CONFLICT (id) DO UPDATE SET
			tenant_id = EXCLUDED.tenant_id,
			person_id = EXCLUDED.person_id,
			created_at = EXCLUDED.created_at,
			updated_at = EXCLUDED.updated_at,
			deleted_at = EXCLUDED.deleted_at
		WHERE (m.tenant_id, m.person_id, m.created_at, m.updated_at, m.deleted_at)
			IS DISTINCT FROM
			(EXCLUDED.tenant_id, EXCLUDED.person_id, EXCLUDED.created_at, EXCLUDED.updated_at, EXCLUDED.deleted_at)
		RETURNING m.id
	),
	profiles AS (
		INSERT INTO users.staff_employment_profiles AS p
			(membership_id, tenant_id, staff_notes, employment_type, work_time_model_id,
			 personnel_number, rotation_anchor_date, birthday_display_opt_out)
		SELECT id, tenant_id, staff_notes, employment_type, work_time_model_id,
		       personnel_number, rotation_anchor_date, birthday_display_opt_out
		FROM eligible
		ON CONFLICT (membership_id) DO UPDATE SET
			tenant_id = EXCLUDED.tenant_id,
			staff_notes = EXCLUDED.staff_notes,
			employment_type = EXCLUDED.employment_type,
			work_time_model_id = EXCLUDED.work_time_model_id,
			personnel_number = EXCLUDED.personnel_number,
			rotation_anchor_date = EXCLUDED.rotation_anchor_date,
			birthday_display_opt_out = EXCLUDED.birthday_display_opt_out
		WHERE (p.tenant_id, p.staff_notes, p.employment_type, p.work_time_model_id,
		       p.personnel_number, p.rotation_anchor_date, p.birthday_display_opt_out)
			IS DISTINCT FROM
			(EXCLUDED.tenant_id, EXCLUDED.staff_notes, EXCLUDED.employment_type, EXCLUDED.work_time_model_id,
			 EXCLUDED.personnel_number, EXCLUDED.rotation_anchor_date, EXCLUDED.birthday_display_opt_out)
		RETURNING p.membership_id AS id
	)
	SELECT (SELECT count(*) FROM batch) AS scanned,
	       (SELECT coalesce(max(id), ?) FROM batch) AS last_id,
	       (SELECT count(*) FROM batch) - (SELECT count(*) FROM eligible) AS rejected,
	       (SELECT count(*) FROM (SELECT id FROM memberships UNION SELECT id FROM profiles) AS written) AS copied`

// staffOwnerSourceProjection and staffOwnerTargetProjection are the canonical
// row shapes compared during verification. Column names and order must match
// so that to_jsonb renders identical text for identical data.
const staffOwnerSourceProjection = `
	SELECT s.id, s.tenant_id, s.person_id, s.created_at, s.updated_at, s.deleted_at,
	       s.staff_notes, s.employment_type, s.work_time_model_id, s.personnel_number,
	       s.rotation_anchor_date, s.birthday_display_opt_out
	FROM users.staff AS s
	WHERE s.tenant_id = ?`

const staffOwnerTargetProjection = `
	SELECT m.id, m.tenant_id, m.person_id, m.created_at, m.updated_at, m.deleted_at,
	       p.staff_notes, p.employment_type, p.work_time_model_id, p.personnel_number,
	       p.rotation_anchor_date, p.birthday_display_opt_out
	FROM users.staff_school_memberships AS m
	JOIN users.staff_employment_profiles AS p ON p.membership_id = m.id AND p.tenant_id = m.tenant_id
	WHERE m.tenant_id = ?`

const staffOwnerSourceChecksum = `
	SELECT count(*), encode(sha256(coalesce(string_agg(sha256(convert_to(to_jsonb(r)::text, 'UTF8')), ''::bytea ORDER BY r.id), ''::bytea)), 'hex')
	FROM (` + staffOwnerSourceProjection + `) AS r`

const staffOwnerTargetChecksum = `
	SELECT count(*), encode(sha256(coalesce(string_agg(sha256(convert_to(to_jsonb(r)::text, 'UTF8')), ''::bytea ORDER BY r.id), ''::bytea)), 'hex')
	FROM (` + staffOwnerTargetProjection + `) AS r`

const staffOwnerMismatch = `
	SELECT count(*) FILTER (WHERE to_jsonb(s) IS DISTINCT FROM to_jsonb(t)),
	       min(s.updated_at) FILTER (WHERE to_jsonb(s) IS DISTINCT FROM to_jsonb(t))
	FROM (` + staffOwnerSourceProjection + `) AS s
	FULL JOIN (` + staffOwnerTargetProjection + `) AS t ON t.id = s.id`

// RunStaffOwnerBackfill copies users.staff into users.staff_school_memberships
// and users.staff_employment_profiles for every school, in deterministic
// tenant/id batches with a persisted high-water mark. Each batch commits on its
// own and is idempotent; interrupting the run and calling it again resumes at
// the checkpoint. After each full pass the tenant is verified by count,
// canonical checksum and row-wise mismatch. A tenant becomes stable once a
// pass changes nothing and verification matches. The old table is never
// modified.
func RunStaffOwnerBackfill(ctx context.Context, db *bun.DB, opts StaffOwnerBackfillOptions) (*StaffOwnerBackfillReport, error) {
	if db == nil {
		return nil, errors.New("staff owner backfill: database is required")
	}
	opts = opts.withDefaults()
	release, err := lockStaffOwnerBackfill(ctx, db)
	if err != nil {
		return nil, err
	}
	defer release()
	if err := assertStaffSourceIsBaseTable(ctx, db); err != nil {
		return nil, err
	}
	var tenantIDs []int64
	if err := db.NewRaw(`SELECT id FROM platform.schools ORDER BY id`).Scan(ctx, &tenantIDs); err != nil {
		return nil, fmt.Errorf("staff owner backfill: list schools: %w", err)
	}
	// A failing school must not leave the remaining schools unvisited: each
	// keeps its own checkpoint, so the others proceed and the failures are
	// reported together. Cancellation stops the run immediately.
	var failures []error
	for _, tenantID := range tenantIDs {
		if err := runStaffOwnerTenant(ctx, db, opts, tenantID); err != nil {
			if ctx.Err() != nil {
				return nil, err
			}
			opts.Logger.Error("staff owner backfill tenant failed",
				"tenant_id", tenantID,
				"error", err)
			failures = append(failures, err)
		}
	}
	// The membership sequence only has to stay above the preserved
	// identities; nothing allocates from it before Cutover, which owns the
	// hand-over from the users.staff sequence.
	if _, err := db.ExecContext(ctx, `
		SELECT setval('users.staff_school_memberships_id_seq', GREATEST(
			COALESCE((SELECT max(id) FROM users.staff_school_memberships), 1),
			(SELECT last_value FROM users.staff_school_memberships_id_seq),
			(SELECT last_value FROM users.staff_id_seq)), true)`); err != nil {
		return nil, fmt.Errorf("staff owner backfill: align membership sequence: %w", err)
	}
	report, err := StaffOwnerBackfillStatus(ctx, db)
	if err != nil {
		return nil, err
	}
	opts.Logger.Info("staff owner backfill finished",
		"tenants", len(report.Tenants),
		"stable", report.Stable(),
		"unstable_tenants", report.Unstable(),
		"failed_tenants", len(failures))
	return report, errors.Join(failures...)
}

type staffOwnerTenantRun struct {
	db        *bun.DB
	opts      StaffOwnerBackfillOptions
	tenantID  int64
	durations []time.Duration
	poolWait  time.Duration
}

func runStaffOwnerTenant(ctx context.Context, db *bun.DB, opts StaffOwnerBackfillOptions, tenantID int64) error {
	run := &staffOwnerTenantRun{db: db, opts: opts, tenantID: tenantID}
	cp, err := run.loadCheckpoint(ctx)
	if err != nil {
		return err
	}
	if cp.PassCompleted {
		// The persisted pass is complete (stable, or left at the pass
		// limit). Re-read the whole tenant so old rows changed since then
		// are found by the copy, not only by verification.
		if err := run.startPass(ctx, cp); err != nil {
			return err
		}
	}
	for pass := 1; ; pass++ {
		if err := run.copyPass(ctx, cp); err != nil {
			return err
		}
		if err := run.removeOrphans(ctx, cp); err != nil {
			return err
		}
		if err := run.verify(ctx, cp); err != nil {
			return err
		}
		if cp.Stable {
			return nil
		}
		if pass >= opts.MaxPasses {
			// Keep the completed pass and its high-water mark; the next run
			// re-reads from a fresh pass without discarding evidence here.
			break
		}
		if err := run.startPass(ctx, cp); err != nil {
			return err
		}
	}
	opts.Logger.Warn("staff owner backfill tenant not stable after pass limit",
		"tenant_id", tenantID,
		"pass", cp.Pass,
		"mismatch_count", cp.MismatchCount,
		"rows_rejected", cp.RowsRejected)
	return nil
}

func (r *staffOwnerTenantRun) loadCheckpoint(ctx context.Context) (*StaffOwnerBackfillCheckpoint, error) {
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO platform.storage_backfill_checkpoints (backfill, tenant_id)
		VALUES (?, ?) ON CONFLICT (backfill, tenant_id) DO NOTHING`, StaffOwnerBackfillName, r.tenantID); err != nil {
		return nil, fmt.Errorf("staff owner backfill: init checkpoint for tenant %d: %w", r.tenantID, err)
	}
	cp := new(StaffOwnerBackfillCheckpoint)
	if err := r.db.NewSelect().Model(cp).
		Where("backfill = ? AND tenant_id = ?", StaffOwnerBackfillName, r.tenantID).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("staff owner backfill: load checkpoint for tenant %d: %w", r.tenantID, err)
	}
	return cp, nil
}

func (r *staffOwnerTenantRun) copyPass(ctx context.Context, cp *StaffOwnerBackfillCheckpoint) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		batch, err := r.copyBatch(ctx, cp)
		if err != nil {
			return err
		}
		if r.opts.afterBatch != nil {
			if err := r.opts.afterBatch(r.tenantID, batch); err != nil {
				return err
			}
		}
		if batch.Scanned < int64(r.opts.BatchSize) {
			return nil
		}
	}
}

// copyBatch runs one batch with retries. Deadlocks, serialization failures
// and lock timeouts retry the same batch; a unique violation restarts the
// pass from zero because the target can only accept a rejoined person after
// the earlier, now soft-deleted, row has been re-read.
func (r *staffOwnerTenantRun) copyBatch(ctx context.Context, cp *StaffOwnerBackfillCheckpoint) (result staffOwnerBatch, resultErr error) {
	var pending staffOwnerRetries
	defer func() {
		if resultErr == nil {
			return
		}
		// Failed attempts rolled back their data checkpoint, not their observed
		// contention. A graceful cancellation still gets a bounded flush.
		flushCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_, err := r.db.ExecContext(flushCtx, `UPDATE platform.storage_backfill_checkpoints SET
			batches_retried = batches_retried + ?, deadlocks = deadlocks + ?,
			serialization_failures = serialization_failures + ?, lock_timeouts = lock_timeouts + ?,
			lock_wait_ms = lock_wait_ms + ?, updated_at = now()
			WHERE backfill = ? AND tenant_id = ?`, pending.retried, pending.deadlocks, pending.serialization,
			pending.lockTimeouts, float64(pending.lockWait)/float64(time.Millisecond), StaffOwnerBackfillName, r.tenantID)
		if err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("persist failed batch telemetry: %w", err))
		}
	}()
	attempt, restarts := 1, 0
	for {
		started := time.Now()
		waitBefore := r.db.DB.Stats().WaitDuration
		batch, err := r.tryCopyBatch(ctx, cp, attempt, &pending)
		r.poolWait += r.db.DB.Stats().WaitDuration - waitBefore
		if err == nil {
			batch.Attempts = attempt
			batch.Duration = time.Since(started)
			r.durations = append(r.durations, batch.Duration)
			cp.BatchesCompleted++
			cp.BatchesRetried += pending.retried
			cp.Deadlocks += pending.deadlocks
			cp.SerializationFailures += pending.serialization
			cp.LockTimeouts += pending.lockTimeouts
			cp.LockWaitMs += float64(pending.lockWait) / float64(time.Millisecond)
			return batch, nil
		}
		code := sqlState(err)
		if !pending.record(code) {
			return staffOwnerBatch{}, fmt.Errorf("staff owner backfill: tenant %d batch after id %d: %w", r.tenantID, cp.HighWaterID, err)
		}
		restart := code == sqlStateUniqueViolation
		if restart {
			// Persist the rewind so a crash before the next commit resumes
			// the restarted pass instead of the stale mark. Restarts have
			// their own budget: each one is caused by a source change, not
			// by contention on the same batch.
			restarts++
			if restarts > r.opts.MaxAttempts {
				return staffOwnerBatch{}, fmt.Errorf("staff owner backfill: tenant %d restarted the pass %d times without converging: %w", r.tenantID, restarts-1, err)
			}
			if err := r.rewindPass(ctx, cp); err != nil {
				return staffOwnerBatch{}, err
			}
		} else {
			attempt++
			if attempt > r.opts.MaxAttempts {
				return staffOwnerBatch{}, fmt.Errorf("staff owner backfill: tenant %d batch after id %d gave up after %d attempts: %w", r.tenantID, cp.HighWaterID, attempt-1, err)
			}
		}
		r.opts.Logger.Warn("staff owner backfill batch retry",
			"tenant_id", r.tenantID,
			"attempt", attempt,
			"restarts", restarts,
			"sqlstate", code,
			"restart_pass", restart)
		select {
		case <-ctx.Done():
			return staffOwnerBatch{}, ctx.Err()
		case <-time.After(staffOwnerRetryBackoff * time.Duration(attempt+restarts)):
		}
		pending.retried++
	}
}

// PostgreSQL SQLSTATE classes the batch loop reacts to.
const (
	sqlStateUniqueViolation      = "23505"
	sqlStateSerializationFailure = "40001"
	sqlStateDeadlockDetected     = "40P01"
	sqlStateLockNotAvailable     = "55P03"
)

// staffOwnerRetries accumulates the transient failures of one batch until
// its successful commit persists them with the checkpoint.
type staffOwnerRetries struct {
	retried, deadlocks, serialization, lockTimeouts int64
	lockWait                                        time.Duration
}

// record classifies a failed attempt and reports whether it may be retried.
func (p *staffOwnerRetries) record(code string) bool {
	switch code {
	case sqlStateDeadlockDetected:
		p.deadlocks++
	case sqlStateSerializationFailure:
		p.serialization++
	case sqlStateLockNotAvailable:
		p.lockTimeouts++
	case sqlStateUniqueViolation:
	default:
		return false
	}
	return true
}

func (r *staffOwnerTenantRun) rewindPass(ctx context.Context, cp *StaffOwnerBackfillCheckpoint) error {
	// A physically deleted row cannot be re-read on the next pass. Remove
	// its blocking membership before retrying a replacement for that person.
	return r.reconcileOrphans(ctx, cp, true)
}

func (r *staffOwnerTenantRun) tryCopyBatch(ctx context.Context, cp *StaffOwnerBackfillCheckpoint, attempt int, pending *staffOwnerRetries) (staffOwnerBatch, error) {
	var batch staffOwnerBatch
	conn, err := r.db.Conn(ctx)
	if err != nil {
		return batch, err
	}
	defer func() { _ = conn.Close() }()
	var pid int
	if err := conn.NewRaw(`SELECT pg_backend_pid()`).Scan(ctx, &pid); err != nil {
		return batch, err
	}
	stopMonitor, _, err := monitorBackfillLocks(ctx, r.db, pid)
	if err != nil {
		return batch, err
	}
	defer func() { _, _ = stopMonitor() }()
	observed := false
	err = conn.RunInTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `SET LOCAL lock_timeout = '5s'; SET LOCAL statement_timeout = '60s'`); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `SELECT tenant_id FROM platform.storage_backfill_checkpoints
			WHERE backfill = ? AND tenant_id = ? FOR UPDATE`, StaffOwnerBackfillName, r.tenantID); err != nil {
			return err
		}
		if err := tx.NewRaw(staffOwnerCopyBatch, r.tenantID, cp.HighWaterID, r.opts.BatchSize, cp.HighWaterID).
			Scan(ctx, &batch.Scanned, &batch.LastID, &batch.Rejected, &batch.Copied); err != nil {
			return err
		}
		batch.Skipped = batch.Scanned - batch.Rejected - batch.Copied
		if r.opts.injectFault != nil {
			if err := r.opts.injectFault(ctx, tx, r.tenantID, attempt); err != nil {
				return err
			}
		}
		lockWait, monitorErr := stopMonitor()
		pending.lockWait += lockWait
		observed = true
		if monitorErr != nil {
			return monitorErr
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE platform.storage_backfill_checkpoints SET
				high_water_id = ?, pass_writes = pass_writes + ?, stable = false,
				rows_scanned = rows_scanned + ?, rows_copied = rows_copied + ?,
				rows_skipped = rows_skipped + ?, rows_rejected = rows_rejected + ?,
				batches_completed = batches_completed + 1, batches_retried = batches_retried + ?,
				deadlocks = deadlocks + ?, serialization_failures = serialization_failures + ?,
				lock_timeouts = lock_timeouts + ?, lock_wait_ms = lock_wait_ms + ?, updated_at = now()
			WHERE backfill = ? AND tenant_id = ?`,
			batch.LastID, batch.Copied, batch.Scanned, batch.Copied, batch.Skipped, batch.Rejected,
			pending.retried, pending.deadlocks, pending.serialization, pending.lockTimeouts,
			float64(pending.lockWait)/float64(time.Millisecond), StaffOwnerBackfillName, r.tenantID); err != nil {
			return err
		}
		return nil
	})
	if !observed {
		lockWait, monitorErr := stopMonitor()
		pending.lockWait += lockWait
		err = errors.Join(err, monitorErr)
	}
	if err != nil {
		return staffOwnerBatch{}, err
	}
	cp.HighWaterID = batch.LastID
	cp.PassWrites += batch.Copied
	cp.Stable = false
	cp.RowsScanned += batch.Scanned
	cp.RowsCopied += batch.Copied
	cp.RowsSkipped += batch.Skipped
	cp.RowsRejected += batch.Rejected
	return batch, nil
}

// removeOrphans deletes target memberships whose source row was physically
// deleted. Profiles follow through the membership cascade. Soft-deleted source
// rows are copied, not removed, because the membership owns that lifecycle.
func (r *staffOwnerTenantRun) removeOrphans(ctx context.Context, cp *StaffOwnerBackfillCheckpoint) error {
	return r.reconcileOrphans(ctx, cp, false)
}

// Cleanup, source-backed retirements and any conflict rewind commit together
// with their counters. Retired memberships are retained, not deleted.
func (r *staffOwnerTenantRun) reconcileOrphans(ctx context.Context, cp *StaffOwnerBackfillCheckpoint, rewind bool) error {
	var removed int64
	var retired int64
	err := r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `SET LOCAL lock_timeout = '5s'; SET LOCAL statement_timeout = '60s'`); err != nil {
			return err
		}
		if err := tx.NewRaw(`
			WITH removed AS (
				DELETE FROM users.staff_school_memberships AS m
				WHERE m.tenant_id = ?
				  AND NOT EXISTS (SELECT 1 FROM users.staff AS s WHERE s.id = m.id AND s.tenant_id = m.tenant_id)
				RETURNING m.id
			)
			SELECT count(*) FROM removed`, r.tenantID).Scan(ctx, &removed); err != nil {
			return err
		}
		if rewind {
			// The replacement may have a lower ID than its retired predecessor
			// and occupy a different batch. Rewinding alone cannot reach the
			// predecessor before the immediate active-person unique check.
			result, err := tx.ExecContext(ctx, `
				UPDATE users.staff_school_memberships AS m
				SET deleted_at = s.deleted_at, created_at = s.created_at, updated_at = s.updated_at
				FROM users.staff AS s
				WHERE m.tenant_id = ? AND s.tenant_id = m.tenant_id AND s.id = m.id
				  AND s.person_id = m.person_id AND m.deleted_at IS NULL AND s.deleted_at IS NOT NULL
				  AND (s.work_time_model_id IS NULL OR EXISTS (
					SELECT 1 FROM config.work_time_models AS w WHERE w.id = s.work_time_model_id AND w.tenant_id = s.tenant_id))
				  AND EXISTS (SELECT 1 FROM users.staff AS replacement
					WHERE replacement.tenant_id = s.tenant_id AND replacement.person_id = s.person_id
					  AND replacement.id <> s.id AND replacement.deleted_at IS NULL)`, r.tenantID)
			if err != nil {
				return err
			}
			retired, err = result.RowsAffected()
			if err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx, `
			UPDATE platform.storage_backfill_checkpoints SET
				high_water_id = CASE WHEN ? THEN 0 ELSE high_water_id END,
				rows_scanned = rows_scanned + ?, rows_copied = rows_copied + ?,
				rows_removed = rows_removed + ?, pass_writes = pass_writes + ?, updated_at = now()
			WHERE backfill = ? AND tenant_id = ?`, rewind, retired, retired, removed, removed+retired, StaffOwnerBackfillName, r.tenantID)
		return err
	})
	if err != nil {
		return fmt.Errorf("staff owner backfill: tenant %d remove orphans: %w", r.tenantID, err)
	}
	cp.RowsRemoved += removed
	cp.RowsScanned += retired
	cp.RowsCopied += retired
	cp.PassWrites += removed + retired
	if rewind {
		cp.HighWaterID = 0
	}
	return nil
}

// verify compares per-tenant counts, canonical checksums and row-wise
// mismatches between the old table and the joined targets, then persists the
// evidence. The pass is stable when it changed nothing and everything matches.
func (r *staffOwnerTenantRun) verify(ctx context.Context, cp *StaffOwnerBackfillCheckpoint) error {
	return r.db.RunInTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `SET LOCAL TIME ZONE 'UTC'; SET LOCAL lock_timeout = '5s'; SET LOCAL statement_timeout = '60s'`); err != nil {
			return err
		}
		return r.verifySnapshot(ctx, tx, cp)
	})
}

func (r *staffOwnerTenantRun) verifySnapshot(ctx context.Context, tx bun.Tx, cp *StaffOwnerBackfillCheckpoint) error {
	var source, target struct {
		Count    int64
		Checksum string
	}
	if err := tx.NewRaw(staffOwnerSourceChecksum, r.tenantID).
		Scan(ctx, &source.Count, &source.Checksum); err != nil {
		return fmt.Errorf("staff owner backfill: tenant %d source checksum: %w", r.tenantID, err)
	}
	if r.opts.afterSourceVerification != nil {
		if err := r.opts.afterSourceVerification(ctx, tx); err != nil {
			return err
		}
	}
	if err := tx.NewRaw(staffOwnerTargetChecksum, r.tenantID).
		Scan(ctx, &target.Count, &target.Checksum); err != nil {
		return fmt.Errorf("staff owner backfill: tenant %d target checksum: %w", r.tenantID, err)
	}
	var mismatches int64
	var oldest sql.NullTime
	if err := tx.NewRaw(staffOwnerMismatch, r.tenantID, r.tenantID).Scan(ctx, &mismatches, &oldest); err != nil {
		return fmt.Errorf("staff owner backfill: tenant %d mismatches: %w", r.tenantID, err)
	}
	var now time.Time
	if err := tx.NewRaw(`SELECT clock_timestamp(), pg_current_snapshot()::text`).Scan(ctx, &now, &cp.VerificationSnapshot); err != nil {
		return fmt.Errorf("staff owner backfill: verification snapshot: %w", err)
	}
	cp.SourceCount, cp.SourceChecksum = source.Count, source.Checksum
	cp.TargetCount, cp.TargetChecksum = target.Count, target.Checksum
	cp.MismatchCount = mismatches
	cp.OldestUnmigratedAt = nil
	if oldest.Valid {
		cp.OldestUnmigratedAt = &oldest.Time
	}
	cp.VerifiedAt = &now
	cp.PassCompleted = true
	cp.Stable = cp.PassWrites == 0 && cp.Verified()
	if cp.Stable {
		cp.StableAt = &now
	}
	p95, maxDuration := r.batchPercentiles()
	if maxDuration > cp.BatchMaxMs {
		cp.BatchMaxMs = maxDuration
	}
	if len(r.durations) > 0 {
		cp.BatchP95Ms = p95
	}
	cp.PoolWaitMs += r.poolWait.Milliseconds()
	r.durations, r.poolWait = nil, 0
	if _, err := tx.ExecContext(ctx, `
		UPDATE platform.storage_backfill_checkpoints SET
			source_count = ?, target_count = ?, source_checksum = ?, target_checksum = ?,
			mismatch_count = ?, oldest_unmigrated_at = ?, verified_at = ?, stable = ?, stable_at = ?,
			batch_p95_ms = ?, batch_max_ms = ?, pool_wait_ms = ?, verification_snapshot = ?, pass_completed = true, updated_at = now()
		WHERE backfill = ? AND tenant_id = ?`,
		cp.SourceCount, cp.TargetCount, cp.SourceChecksum, cp.TargetChecksum,
		cp.MismatchCount, cp.OldestUnmigratedAt, cp.VerifiedAt, cp.Stable, cp.StableAt,
		cp.BatchP95Ms, cp.BatchMaxMs, cp.PoolWaitMs, cp.VerificationSnapshot, StaffOwnerBackfillName, r.tenantID); err != nil {
		return fmt.Errorf("staff owner backfill: tenant %d persist verification: %w", r.tenantID, err)
	}
	r.opts.Logger.Info("staff owner backfill pass verified",
		"tenant_id", r.tenantID,
		"pass", cp.Pass,
		"stable", cp.Stable,
		"source_count", cp.SourceCount,
		"target_count", cp.TargetCount,
		"mismatch_count", cp.MismatchCount,
		"rows_copied", cp.RowsCopied,
		"rows_rejected", cp.RowsRejected,
		"rows_removed", cp.RowsRemoved)
	return nil
}

func (r *staffOwnerTenantRun) startPass(ctx context.Context, cp *StaffOwnerBackfillCheckpoint) error {
	cp.Pass++
	cp.HighWaterID = 0
	cp.PassWrites = 0
	cp.PassCompleted = false
	cp.Stable = false
	if _, err := r.db.ExecContext(ctx, `
		UPDATE platform.storage_backfill_checkpoints SET
			pass = ?, high_water_id = 0, pass_writes = 0, pass_completed = false, stable = false, updated_at = now()
		WHERE backfill = ? AND tenant_id = ?`, cp.Pass, StaffOwnerBackfillName, r.tenantID); err != nil {
		return fmt.Errorf("staff owner backfill: tenant %d start pass %d: %w", r.tenantID, cp.Pass, err)
	}
	return nil
}

func (r *staffOwnerTenantRun) batchPercentiles() (p95, maxMs int64) {
	if len(r.durations) == 0 {
		return 0, 0
	}
	sorted := slices.Clone(r.durations)
	slices.Sort(sorted)
	index := max((len(sorted)*95+99)/100, 1)
	return sorted[index-1].Milliseconds(), sorted[len(sorted)-1].Milliseconds()
}

// StaffOwnerBackfillStatus reads every tenant checkpoint without changing
// anything. Schools without a checkpoint have not been visited yet.
func StaffOwnerBackfillStatus(ctx context.Context, db *bun.DB) (*StaffOwnerBackfillReport, error) {
	if db == nil {
		return nil, errors.New("staff owner backfill: database is required")
	}
	report := &StaffOwnerBackfillReport{Tenants: []StaffOwnerBackfillCheckpoint{}, MissingTenants: []int64{}}
	if err := db.NewSelect().Model(&report.Tenants).
		Where("backfill = ?", StaffOwnerBackfillName).
		OrderExpr("tenant_id").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("staff owner backfill: read checkpoints: %w", err)
	}
	if err := db.NewRaw(`
		SELECT s.id FROM platform.schools AS s
		WHERE NOT EXISTS (
			SELECT 1 FROM platform.storage_backfill_checkpoints AS c
			WHERE c.backfill = ? AND c.tenant_id = s.id
		)
		ORDER BY s.id`, StaffOwnerBackfillName).Scan(ctx, &report.MissingTenants); err != nil {
		return nil, fmt.Errorf("staff owner backfill: list unvisited schools: %w", err)
	}
	return report, nil
}

// ResetStaffOwnerBackfill discards every target row and checkpoint so the
// backfill restarts from zero. It refuses once users.staff is no longer the
// authoritative base table, because after Cutover the targets hold live data.
func ResetStaffOwnerBackfill(ctx context.Context, db *bun.DB) error {
	if db == nil {
		return errors.New("staff owner backfill: database is required")
	}
	release, err := lockStaffOwnerBackfill(ctx, db)
	if err != nil {
		return err
	}
	defer release()
	if err := assertStaffSourceIsBaseTable(ctx, db); err != nil {
		return err
	}
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			TRUNCATE users.staff_employment_profiles, users.staff_school_memberships;
			DELETE FROM platform.storage_backfill_checkpoints WHERE backfill = ?;`, StaffOwnerBackfillName); err != nil {
			return fmt.Errorf("staff owner backfill: reset targets: %w", err)
		}
		return nil
	})
}

// assertStaffSourceIsBaseTable guards every target-only write: Cutover
// replaces users.staff with a compatibility view, after which the targets
// are authoritative and must not be overwritten from the old shape.
func assertStaffSourceIsBaseTable(ctx context.Context, db bun.IDB) error {
	var kind string
	if err := db.NewRaw(`SELECT relkind::text FROM pg_class WHERE oid = 'users.staff'::regclass`).Scan(ctx, &kind); err != nil {
		return fmt.Errorf("staff owner backfill: inspect users.staff: %w", err)
	}
	if kind != "r" {
		return fmt.Errorf("staff owner backfill: users.staff is not a base table (relkind %q); the targets are authoritative after Cutover", kind)
	}
	return nil
}

func sqlState(err error) string {
	if pgErr, ok := errors.AsType[pgdriver.Error](err); ok {
		return pgErr.Field('C')
	}
	return ""
}
