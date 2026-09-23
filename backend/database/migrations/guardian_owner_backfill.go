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
)

// This runner deliberately repeats the batch/retry/checkpoint shape of the
// Staff (#2752) and Student (#2758) backfills instead of sharing an engine with
// them. It differs in the copy statement, the canonical projections, the
// conflict recovery (the relationship table carries three unique keys that a
// source edit can move between rows) and the verification set (guardian access
// binding), and a shared engine would couple the shipped migrations to every
// later edit of a one-shot copy. Only what is genuinely identical is shared:
// lockStorageBackfill, monitorBackfillLocks and the SQLSTATE constants.

// GuardianOwnerBackfillName keys the guardian People/Care Plan/Identity
// backfill in the shared checkpoint table.
const GuardianOwnerBackfillName = "guardian-owner"

const (
	defaultGuardianOwnerBatchSize   = 500
	defaultGuardianOwnerMaxPasses   = 5
	defaultGuardianOwnerMaxAttempts = 5
	guardianOwnerRetryBackoff       = 50 * time.Millisecond
)

// GuardianOwnerBackfillOptions tunes RunGuardianOwnerBackfill. Zero values
// select the defaults. The unexported seams exist for tests that interrupt the
// run at batch boundaries or inject transient database failures.
type GuardianOwnerBackfillOptions struct {
	// BatchSize bounds the number of users.students_guardians rows read per
	// transaction.
	BatchSize int
	// MaxPasses bounds the re-read passes per tenant in one run. A tenant is
	// stable when a full pass changes nothing and verification matches.
	MaxPasses int
	// MaxAttempts bounds two independent budgets: the retries of one batch
	// after a deadlock, serialization failure or lock timeout, and the pass
	// restarts a unique-conflict rewind may take.
	MaxAttempts int
	Logger      *slog.Logger

	// afterBatch runs after each committed batch. Returning an error stops the
	// run; the committed checkpoint stays behind for the next run.
	afterBatch func(tenantID int64, batch guardianOwnerBatch) error
	// injectFault runs inside the batch transaction before commit.
	injectFault func(ctx context.Context, tx bun.Tx, tenantID int64, attempt int) error
	// afterSourceVerification permits a deterministic concurrent source edit.
	afterSourceVerification func(context.Context, bun.Tx) error
}

func (o GuardianOwnerBackfillOptions) withDefaults() GuardianOwnerBackfillOptions {
	if o.BatchSize <= 0 {
		o.BatchSize = defaultGuardianOwnerBatchSize
	}
	if o.MaxPasses <= 0 {
		o.MaxPasses = defaultGuardianOwnerMaxPasses
	}
	if o.MaxAttempts <= 0 {
		o.MaxAttempts = defaultGuardianOwnerMaxAttempts
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	return o
}

// GuardianOwnerBackfillCheckpoint is the persisted per-tenant state and
// runtime evidence of the backfill. Counters are cumulative across runs and
// passes; Pass, HighWaterID and PassWrites describe the pass in progress.
type GuardianOwnerBackfillCheckpoint struct {
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

	SourceCount    int64  `bun:"source_count" json:"source_count"`
	TargetCount    int64  `bun:"target_count" json:"target_count"`
	SourceChecksum string `bun:"source_checksum" json:"source_checksum"`
	TargetChecksum string `bun:"target_checksum" json:"target_checksum"`
	// MismatchCount counts source rows the three targets do not reproduce
	// column for column.
	MismatchCount int64 `bun:"mismatch_count" json:"mismatch_count"`
	// GuardianAccessMismatchCount counts relationships whose Identity row
	// does not carry the guardian's current account binding and parent-portal
	// permissions. The binding lives on users.guardian_profiles, whose edits
	// do not touch the source row, so it is verified on its own.
	GuardianAccessMismatchCount int64 `bun:"guardian_access_mismatch_count" json:"guardian_access_mismatch_count"`
	// RowsVisibleToTenant is how many of the three targets' rows the
	// phoenix_tenant role saw for this school at the last verification. The
	// copy runs as superuser and bypasses the policies, so this is the only
	// evidence in the checkpoint that they still hold.
	RowsVisibleToTenant int64      `bun:"rows_visible_to_tenant" json:"rows_visible_to_tenant"`
	OldestUnmigratedAt  *time.Time `bun:"oldest_unmigrated_at" json:"oldest_unmigrated_at,omitempty"`

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
// checksums, no mismatched or orphaned rows and every guardian access row
// bound to the guardian's current account.
func (c GuardianOwnerBackfillCheckpoint) Verified() bool {
	return c.VerifiedAt != nil && c.MismatchCount == 0 && c.GuardianAccessMismatchCount == 0 &&
		c.SourceCount == c.TargetCount && c.SourceChecksum == c.TargetChecksum
}

// OldestUnmigratedAge is the age of the oldest source change not yet
// reflected in the targets at the last verification, or zero.
func (c GuardianOwnerBackfillCheckpoint) OldestUnmigratedAge(now time.Time) time.Duration {
	if c.OldestUnmigratedAt == nil {
		return 0
	}
	return now.Sub(*c.OldestUnmigratedAt)
}

// GuardianOwnerBackfillReport is the per-tenant state after a run or status read.
type GuardianOwnerBackfillReport struct {
	Tenants []GuardianOwnerBackfillCheckpoint `json:"tenants"`
	// MissingTenants lists schools without any checkpoint: they have never
	// been visited, for example after a reset or when a school was created
	// after the last run.
	MissingTenants []int64 `json:"missing_tenants"`
}

// Stable reports whether every school has a verified, stable checkpoint. This
// is the Cutover precondition; it says nothing about writes after StableAt.
func (r *GuardianOwnerBackfillReport) Stable() bool {
	return r != nil && len(r.Unstable()) == 0
}

// Unstable returns the schools that still block Cutover: unvisited ones and
// those whose last pass changed rows or failed verification.
func (r *GuardianOwnerBackfillReport) Unstable() []int64 {
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

type guardianOwnerBatch struct {
	Scanned  int64
	LastID   int64
	Rejected int64
	Copied   int64
	Skipped  int64
	Attempts int
	Duration time.Duration
}

// guardianOwnerEligibleBatch selects one keyset batch of source rows joined
// with the guardian's account binding. Two kinds of row are rejected rather
// than copied; both count as rejected and keep the tenant unverified until the
// source is corrected:
//
//   - A row without a same-tenant guardian profile. users.students_guardians
//     already carries composite foreign keys for both the student and the
//     guardian, so this cannot exist once those constraints are validated; the
//     check keeps a forced row from being laundered into Identity storage with
//     a NULL binding.
//   - A primary row of a child who has more than one primary. The old table
//     enforces the single primary only through a trigger on writes of
//     is_primary; moving a primary row to another child with an UPDATE of
//     student_id leaves two primaries, which the target's partial unique index
//     refuses. Copying either would make the run rewind forever, so the copy
//     leaves both out and the mismatch reports them. Payer needs no such
//     check: the old table has the same partial unique index as the target.
const guardianOwnerEligibleBatch = `
	batch AS (
		SELECT sg.id, sg.tenant_id, sg.student_id, sg.guardian_profile_id, sg.relationship_type,
		       sg.guardian_role, sg.is_primary, sg.is_emergency_contact, sg.emergency_priority,
		       sg.is_payer, sg.can_pickup, sg.pickup_notes, sg.permissions, sg.created_at, sg.updated_at
		FROM users.students_guardians AS sg
		WHERE sg.tenant_id = ? AND sg.id > ?
		ORDER BY sg.id
		LIMIT ?
	),
	eligible AS (
		SELECT b.*, g.account_id
		FROM batch AS b
		JOIN users.guardian_profiles AS g ON g.id = b.guardian_profile_id AND g.tenant_id = b.tenant_id
		WHERE NOT (b.is_primary AND EXISTS (
			SELECT 1 FROM users.students_guardians AS o
			WHERE o.tenant_id = b.tenant_id AND o.student_id = b.student_id AND o.is_primary AND o.id <> b.id))
	)`

// guardianOwnerCopyBatch copies one batch into all three targets and returns
// the batch statistics. The relationship keeps the users.students_guardians id,
// so every foreign key that names a link today (consents, meal participation)
// points at the same number after Cutover. The pickup permission and the
// access row hang off that id through relationship_id; their own ids are not
// part of the contract. All three upserts only touch rows whose content
// differs, so reruns of a completed batch are no-ops and count as skipped.
//
// The Identity row binds the guardian's global account as
// users.guardian_profiles.account_id names it today, so a guardian who gains or
// loses a portal account between two passes is picked up by the next one.
const guardianOwnerCopyBatch = `
	WITH ` + guardianOwnerEligibleBatch + `,
	relationships AS (
		INSERT INTO users.student_guardian_relationships AS r
			(id, tenant_id, student_id, guardian_profile_id, relationship_type, guardian_role,
			 is_primary, is_emergency_contact, emergency_priority, is_payer, created_at, updated_at)
		SELECT id, tenant_id, student_id, guardian_profile_id, relationship_type, guardian_role,
		       is_primary, is_emergency_contact, emergency_priority, is_payer, created_at, updated_at
		FROM eligible
		ON CONFLICT (id) DO UPDATE SET
			tenant_id = EXCLUDED.tenant_id,
			student_id = EXCLUDED.student_id,
			guardian_profile_id = EXCLUDED.guardian_profile_id,
			relationship_type = EXCLUDED.relationship_type,
			guardian_role = EXCLUDED.guardian_role,
			is_primary = EXCLUDED.is_primary,
			is_emergency_contact = EXCLUDED.is_emergency_contact,
			emergency_priority = EXCLUDED.emergency_priority,
			is_payer = EXCLUDED.is_payer,
			created_at = EXCLUDED.created_at,
			updated_at = EXCLUDED.updated_at
		WHERE (r.tenant_id, r.student_id, r.guardian_profile_id, r.relationship_type, r.guardian_role,
		       r.is_primary, r.is_emergency_contact, r.emergency_priority, r.is_payer,
		       r.created_at, r.updated_at)
			IS DISTINCT FROM
			(EXCLUDED.tenant_id, EXCLUDED.student_id, EXCLUDED.guardian_profile_id, EXCLUDED.relationship_type,
			 EXCLUDED.guardian_role, EXCLUDED.is_primary, EXCLUDED.is_emergency_contact,
			 EXCLUDED.emergency_priority, EXCLUDED.is_payer, EXCLUDED.created_at, EXCLUDED.updated_at)
		RETURNING r.id
	),
	pickup AS (
		INSERT INTO users.student_guardian_pickup_permissions AS p
			(tenant_id, relationship_id, can_pickup, pickup_notes, created_at, updated_at)
		SELECT tenant_id, id, can_pickup, pickup_notes, created_at, updated_at
		FROM eligible
		ON CONFLICT (tenant_id, relationship_id) DO UPDATE SET
			can_pickup = EXCLUDED.can_pickup,
			pickup_notes = EXCLUDED.pickup_notes,
			created_at = EXCLUDED.created_at,
			updated_at = EXCLUDED.updated_at
		WHERE (p.can_pickup, p.pickup_notes, p.created_at, p.updated_at)
			IS DISTINCT FROM
			(EXCLUDED.can_pickup, EXCLUDED.pickup_notes, EXCLUDED.created_at, EXCLUDED.updated_at)
		RETURNING p.relationship_id AS id
	),
	access AS (
		INSERT INTO auth.guardian_student_access AS a
			(tenant_id, relationship_id, account_id, permissions, created_at, updated_at)
		SELECT tenant_id, id, account_id, permissions, created_at, updated_at
		FROM eligible
		ON CONFLICT (tenant_id, relationship_id) DO UPDATE SET
			account_id = EXCLUDED.account_id,
			permissions = EXCLUDED.permissions,
			created_at = EXCLUDED.created_at,
			updated_at = EXCLUDED.updated_at
		WHERE (a.account_id, a.permissions, a.created_at, a.updated_at)
			IS DISTINCT FROM
			(EXCLUDED.account_id, EXCLUDED.permissions, EXCLUDED.created_at, EXCLUDED.updated_at)
		RETURNING a.relationship_id AS id
	)
	SELECT (SELECT count(*) FROM batch) AS scanned,
	       (SELECT coalesce(max(id), ?) FROM batch) AS last_id,
	       (SELECT count(*) FROM batch) - (SELECT count(*) FROM eligible) AS rejected,
	       (SELECT count(*) FROM (SELECT id FROM relationships UNION SELECT id FROM pickup
	                              UNION SELECT id FROM access) AS written) AS copied`

// copyGuardianOwnerRows runs one copy step of the backfill statement inside
// the caller's transaction: the rows after highWater, at most limit of them.
// The cutover's final delta (#2756) loops it under its write lock.
func copyGuardianOwnerRows(ctx context.Context, tx bun.Tx, tenantID, highWater int64, limit int) (guardianOwnerBatch, error) {
	var batch guardianOwnerBatch
	err := tx.NewRaw(guardianOwnerCopyBatch, tenantID, highWater, limit, highWater).
		Scan(ctx, &batch.Scanned, &batch.LastID, &batch.Rejected, &batch.Copied)
	return batch, err
}

// RunGuardianOwnerBackfill copies users.students_guardians into
// users.student_guardian_relationships, users.student_guardian_pickup_permissions
// and auth.guardian_student_access for every school, in deterministic tenant/id
// batches with a persisted high-water mark. Each batch commits on its own and
// is idempotent; interrupting the run and calling it again resumes at the
// checkpoint. After each full pass the tenant is verified by count, canonical
// checksum, row-wise mismatch, guardian access binding and tenant row-level
// security. A tenant becomes stable once a pass changes nothing and every
// verification matches. The old table is never modified.
func RunGuardianOwnerBackfill(ctx context.Context, db *bun.DB, opts GuardianOwnerBackfillOptions) (*GuardianOwnerBackfillReport, error) {
	if db == nil {
		return nil, errors.New("guardian owner backfill: database is required")
	}
	opts = opts.withDefaults()
	release, err := lockStorageBackfill(ctx, db, GuardianOwnerBackfillName)
	if err != nil {
		return nil, err
	}
	defer release()
	if err := assertGuardianSourceIsBaseTable(ctx, db); err != nil {
		return nil, err
	}
	var tenantIDs []int64
	if err := db.NewRaw(`SELECT id FROM platform.schools ORDER BY id`).Scan(ctx, &tenantIDs); err != nil {
		return nil, fmt.Errorf("guardian owner backfill: list schools: %w", err)
	}
	// A failing school must not leave the remaining schools unvisited: each
	// keeps its own checkpoint, so the others proceed and the failures are
	// reported together. Cancellation stops the run immediately.
	var failures []error
	for _, tenantID := range tenantIDs {
		if err := runGuardianOwnerTenant(ctx, db, opts, tenantID); err != nil {
			if ctx.Err() != nil {
				return nil, err
			}
			opts.Logger.Error("guardian owner backfill tenant failed",
				"tenant_id", tenantID,
				"error", err)
			failures = append(failures, err)
		}
	}
	// The relationship sequence only has to stay above the preserved
	// identities; nothing allocates from it before Cutover, which owns the
	// hand-over from the users.students_guardians sequence.
	if _, err := db.ExecContext(ctx, `
		SELECT setval('users.student_guardian_relationships_id_seq', GREATEST(
			COALESCE((SELECT max(id) FROM users.student_guardian_relationships), 1),
			(SELECT last_value FROM users.student_guardian_relationships_id_seq),
			(SELECT last_value FROM users.students_guardians_id_seq)), true)`); err != nil {
		return nil, fmt.Errorf("guardian owner backfill: align target sequence: %w", err)
	}
	report, err := GuardianOwnerBackfillStatus(ctx, db)
	if err != nil {
		return nil, err
	}
	opts.Logger.Info("guardian owner backfill finished",
		"tenants", len(report.Tenants),
		"stable", report.Stable(),
		"unstable_tenants", report.Unstable(),
		"failed_tenants", len(failures))
	return report, errors.Join(failures...)
}

type guardianOwnerTenantRun struct {
	db        *bun.DB
	opts      GuardianOwnerBackfillOptions
	tenantID  int64
	durations []time.Duration
	poolWait  time.Duration
}

func runGuardianOwnerTenant(ctx context.Context, db *bun.DB, opts GuardianOwnerBackfillOptions, tenantID int64) error {
	run := &guardianOwnerTenantRun{db: db, opts: opts, tenantID: tenantID}
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
	opts.Logger.Warn("guardian owner backfill tenant not stable after pass limit",
		"tenant_id", tenantID,
		"pass", cp.Pass,
		"mismatch_count", cp.MismatchCount,
		"guardian_access_mismatch_count", cp.GuardianAccessMismatchCount,
		"rows_rejected", cp.RowsRejected)
	return nil
}

func (r *guardianOwnerTenantRun) loadCheckpoint(ctx context.Context) (*GuardianOwnerBackfillCheckpoint, error) {
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO platform.storage_backfill_checkpoints (backfill, tenant_id)
		VALUES (?, ?) ON CONFLICT (backfill, tenant_id) DO NOTHING`, GuardianOwnerBackfillName, r.tenantID); err != nil {
		return nil, fmt.Errorf("guardian owner backfill: init checkpoint for tenant %d: %w", r.tenantID, err)
	}
	cp := new(GuardianOwnerBackfillCheckpoint)
	if err := r.db.NewSelect().Model(cp).
		Where("backfill = ? AND tenant_id = ?", GuardianOwnerBackfillName, r.tenantID).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("guardian owner backfill: load checkpoint for tenant %d: %w", r.tenantID, err)
	}
	return cp, nil
}

func (r *guardianOwnerTenantRun) copyPass(ctx context.Context, cp *GuardianOwnerBackfillCheckpoint) error {
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

// copyBatch runs one batch with retries. Deadlocks, serialization failures and
// lock timeouts retry the same batch; a unique violation restarts the pass from
// zero because the relationship table can only accept a row after the target
// row that still claims its unique key has been removed.
func (r *guardianOwnerTenantRun) copyBatch(ctx context.Context, cp *GuardianOwnerBackfillCheckpoint) (result guardianOwnerBatch, resultErr error) {
	var pending guardianOwnerRetries
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
			pending.lockTimeouts, float64(pending.lockWait)/float64(time.Millisecond), GuardianOwnerBackfillName, r.tenantID)
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
			return guardianOwnerBatch{}, fmt.Errorf("guardian owner backfill: tenant %d batch after id %d: %w", r.tenantID, cp.HighWaterID, err)
		}
		restart := code == sqlStateUniqueViolation
		if restart {
			// Persist the rewind so a crash before the next commit resumes
			// the restarted pass instead of the stale mark. Restarts have
			// their own budget: each one is caused by a source change, not
			// by contention on the same batch.
			restarts++
			if restarts > r.opts.MaxAttempts {
				return guardianOwnerBatch{}, fmt.Errorf("guardian owner backfill: tenant %d restarted the pass %d times without converging: %w", r.tenantID, restarts-1, err)
			}
			if err := r.rewindPass(ctx, cp); err != nil {
				return guardianOwnerBatch{}, err
			}
		} else {
			attempt++
			if attempt > r.opts.MaxAttempts {
				return guardianOwnerBatch{}, fmt.Errorf("guardian owner backfill: tenant %d batch after id %d gave up after %d attempts: %w", r.tenantID, cp.HighWaterID, attempt-1, err)
			}
		}
		r.opts.Logger.Warn("guardian owner backfill batch retry",
			"tenant_id", r.tenantID,
			"attempt", attempt,
			"restarts", restarts,
			"sqlstate", code,
			"restart_pass", restart)
		select {
		case <-ctx.Done():
			return guardianOwnerBatch{}, ctx.Err()
		case <-time.After(guardianOwnerRetryBackoff * time.Duration(attempt+restarts)):
		}
		pending.retried++
	}
}

// guardianOwnerRetries accumulates the transient failures of one batch until
// its successful commit persists them with the checkpoint.
type guardianOwnerRetries struct {
	retried, deadlocks, serialization, lockTimeouts int64
	lockWait                                        time.Duration
}

// record classifies a failed attempt and reports whether it may be retried.
func (p *guardianOwnerRetries) record(code string) bool {
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

// rewindPass releases the unique keys before retrying. The relationship table
// carries three of them: one (student, guardian) pair per school, and at most
// one primary and one payer per child. A conflict means some target row still
// claims a key its own source row no longer holds: the link was deleted and
// recreated under a new id, or the primary/payer flag moved to another
// guardian of the same child and the batch reached the newly flagged row
// before the demoted one. Rewinding alone would meet the same claim on every
// restart, so the stale rows go first and the restarted pass rebuilds them
// from the authoritative source.
func (r *guardianOwnerTenantRun) rewindPass(ctx context.Context, cp *GuardianOwnerBackfillCheckpoint) error {
	return r.reconcileOrphans(ctx, cp, true)
}

func (r *guardianOwnerTenantRun) tryCopyBatch(ctx context.Context, cp *GuardianOwnerBackfillCheckpoint, attempt int, pending *guardianOwnerRetries) (guardianOwnerBatch, error) {
	var batch guardianOwnerBatch
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
			WHERE backfill = ? AND tenant_id = ? FOR UPDATE`, GuardianOwnerBackfillName, r.tenantID); err != nil {
			return err
		}
		if err := tx.NewRaw(guardianOwnerCopyBatch, r.tenantID, cp.HighWaterID, r.opts.BatchSize, cp.HighWaterID).
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
			float64(pending.lockWait)/float64(time.Millisecond), GuardianOwnerBackfillName, r.tenantID); err != nil {
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
		return guardianOwnerBatch{}, err
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

// removeOrphans deletes target relationships whose source row was physically
// deleted. Pickup permissions and access rows follow through the cascade.
func (r *guardianOwnerTenantRun) removeOrphans(ctx context.Context, cp *GuardianOwnerBackfillCheckpoint) error {
	return r.reconcileOrphans(ctx, cp, false)
}

// Cleanup and any conflict rewind commit together with their counters. The
// end-of-pass sweep only drops relationships whose source row is gone; a rewind
// also drops those whose source row now names a different pair or carries a
// different primary/payer flag, because that stale claim on one of the unique
// keys is what rejected the batch. A changed flag alone is no reason to delete:
// the ordinary copy updates it in place, and only a conflict proves the update
// could not run.
func (r *guardianOwnerTenantRun) reconcileOrphans(ctx context.Context, cp *GuardianOwnerBackfillCheckpoint, rewind bool) error {
	var removed int64
	err := r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `SET LOCAL lock_timeout = '5s'; SET LOCAL statement_timeout = '60s'`); err != nil {
			return err
		}
		if err := tx.NewRaw(`
			WITH removed AS (
				DELETE FROM users.student_guardian_relationships AS r
				WHERE r.tenant_id = ?
				  AND NOT EXISTS (
					SELECT 1 FROM users.students_guardians AS sg
					WHERE sg.id = r.id AND sg.tenant_id = r.tenant_id
					  AND (NOT ? OR (sg.student_id = r.student_id AND sg.guardian_profile_id = r.guardian_profile_id
					                 AND sg.is_primary = r.is_primary AND sg.is_payer = r.is_payer)))
				RETURNING r.id
			)
			SELECT count(*) FROM removed`, r.tenantID, rewind).Scan(ctx, &removed); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
			UPDATE platform.storage_backfill_checkpoints SET
				high_water_id = CASE WHEN ? THEN 0 ELSE high_water_id END,
				rows_removed = rows_removed + ?, pass_writes = pass_writes + ?, updated_at = now()
			WHERE backfill = ? AND tenant_id = ?`, rewind, removed, removed, GuardianOwnerBackfillName, r.tenantID)
		return err
	})
	if err != nil {
		return fmt.Errorf("guardian owner backfill: tenant %d remove orphans: %w", r.tenantID, err)
	}
	cp.RowsRemoved += removed
	cp.PassWrites += removed
	if rewind {
		cp.HighWaterID = 0
	}
	return nil
}

func (r *guardianOwnerTenantRun) startPass(ctx context.Context, cp *GuardianOwnerBackfillCheckpoint) error {
	cp.Pass++
	cp.HighWaterID = 0
	cp.PassWrites = 0
	cp.PassCompleted = false
	cp.Stable = false
	if _, err := r.db.ExecContext(ctx, `
		UPDATE platform.storage_backfill_checkpoints SET
			pass = ?, high_water_id = 0, pass_writes = 0, pass_completed = false, stable = false, updated_at = now()
		WHERE backfill = ? AND tenant_id = ?`, cp.Pass, GuardianOwnerBackfillName, r.tenantID); err != nil {
		return fmt.Errorf("guardian owner backfill: tenant %d start pass %d: %w", r.tenantID, cp.Pass, err)
	}
	return nil
}

func (r *guardianOwnerTenantRun) batchPercentiles() (p95, maxMs int64) {
	if len(r.durations) == 0 {
		return 0, 0
	}
	sorted := slices.Clone(r.durations)
	slices.Sort(sorted)
	index := max((len(sorted)*95+99)/100, 1)
	return sorted[index-1].Milliseconds(), sorted[len(sorted)-1].Milliseconds()
}

// GuardianOwnerBackfillStatus reads every tenant checkpoint without changing
// anything. Schools without a checkpoint have not been visited yet.
func GuardianOwnerBackfillStatus(ctx context.Context, db *bun.DB) (*GuardianOwnerBackfillReport, error) {
	if db == nil {
		return nil, errors.New("guardian owner backfill: database is required")
	}
	report := &GuardianOwnerBackfillReport{Tenants: []GuardianOwnerBackfillCheckpoint{}, MissingTenants: []int64{}}
	if err := db.NewSelect().Model(&report.Tenants).
		Where("backfill = ?", GuardianOwnerBackfillName).
		OrderExpr("tenant_id").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("guardian owner backfill: read checkpoints: %w", err)
	}
	if err := db.NewRaw(`
		SELECT s.id FROM platform.schools AS s
		WHERE NOT EXISTS (
			SELECT 1 FROM platform.storage_backfill_checkpoints AS c
			WHERE c.backfill = ? AND c.tenant_id = s.id
		)
		ORDER BY s.id`, GuardianOwnerBackfillName).Scan(ctx, &report.MissingTenants); err != nil {
		return nil, fmt.Errorf("guardian owner backfill: list unvisited schools: %w", err)
	}
	return report, nil
}

// ResetGuardianOwnerBackfill discards every target row and checkpoint so the
// backfill restarts from zero. It refuses once users.students_guardians is no
// longer the authoritative base table, because after Cutover the targets hold
// live data.
func ResetGuardianOwnerBackfill(ctx context.Context, db *bun.DB) error {
	if db == nil {
		return errors.New("guardian owner backfill: database is required")
	}
	release, err := lockStorageBackfill(ctx, db, GuardianOwnerBackfillName)
	if err != nil {
		return err
	}
	defer release()
	if err := assertGuardianSourceIsBaseTable(ctx, db); err != nil {
		return err
	}
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			TRUNCATE auth.guardian_student_access, users.student_guardian_pickup_permissions, users.student_guardian_relationships;
			DELETE FROM platform.storage_backfill_checkpoints WHERE backfill = ?;`, GuardianOwnerBackfillName); err != nil {
			return fmt.Errorf("guardian owner backfill: reset targets: %w", err)
		}
		return nil
	})
}

// assertGuardianSourceIsBaseTable guards every target-only write: after
// Cutover (#2756) users.students_guardians is only a rollback mirror of the
// targets, which are authoritative and must not be overwritten from the old
// shape. The mirror stays a base table, so the installed compatibility
// triggers are what mark the switch.
func assertGuardianSourceIsBaseTable(ctx context.Context, db bun.IDB) error {
	var kind string
	if err := db.NewRaw(`SELECT relkind::text FROM pg_class WHERE oid = 'users.students_guardians'::regclass`).Scan(ctx, &kind); err != nil {
		return fmt.Errorf("guardian owner backfill: inspect users.students_guardians: %w", err)
	}
	if kind != "r" {
		return fmt.Errorf("guardian owner backfill: users.students_guardians is not a base table (relkind %q); the targets are authoritative after Cutover", kind)
	}
	return requireGuardianStorageBeforeCutover(ctx, db)
}
