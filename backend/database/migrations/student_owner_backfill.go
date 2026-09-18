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
// Staff backfill (staff_owner_backfill.go, #2752) instead of sharing an engine
// with it. The two differ in the copy statement, the eligibility rule, the
// canonical projections, the conflict recovery (Staff retires soft-deleted
// memberships, users.students has no soft deletion) and the verification set
// (Students add the guardian reconciliation and the care-state equivalence),
// and a shared engine would couple the already-shipped 1.15.382 migration to
// every later edit of a one-shot copy. Only what is genuinely identical is
// shared: lockStorageBackfill and the SQLSTATE constants. The cost is a second
// bun model on platform.storage_backfill_checkpoints — the Staff model omits
// guardian_mismatch_count and care_state_mismatch_count on purpose, because
// only this backfill writes them.

// StudentOwnerBackfillName keys the student People/Membership/Care Plan
// backfill in the shared checkpoint table.
const StudentOwnerBackfillName = "student-owner"

const (
	defaultStudentOwnerBatchSize   = 500
	defaultStudentOwnerMaxPasses   = 5
	defaultStudentOwnerMaxAttempts = 5
	studentOwnerRetryBackoff       = 50 * time.Millisecond
)

// StudentOwnerBackfillOptions tunes RunStudentOwnerBackfill. Zero values select
// the defaults. The unexported seams exist for tests that interrupt the run at
// batch boundaries or inject transient database failures.
type StudentOwnerBackfillOptions struct {
	// BatchSize bounds the number of users.students rows read per transaction.
	BatchSize int
	// MaxPasses bounds the re-read passes per tenant in one run. A tenant is
	// stable when a full pass changes nothing and verification matches.
	MaxPasses int
	// MaxAttempts bounds two independent budgets: the retries of one batch
	// after a deadlock, serialization failure or lock timeout, and the pass
	// restarts a unique-conflict rewind may take. A retry re-runs the same
	// batch, a restart re-reads the tenant, so they are counted apart.
	MaxAttempts int
	Logger      *slog.Logger

	// afterBatch runs after each committed batch. Returning an error stops the
	// run; the committed checkpoint stays behind for the next run.
	afterBatch func(tenantID int64, batch studentOwnerBatch) error
	// injectFault runs inside the batch transaction before commit.
	injectFault func(ctx context.Context, tx bun.Tx, tenantID int64, attempt int) error
	// afterSourceVerification permits a deterministic concurrent source edit.
	afterSourceVerification func(context.Context, bun.Tx) error
}

func (o StudentOwnerBackfillOptions) withDefaults() StudentOwnerBackfillOptions {
	if o.BatchSize <= 0 {
		o.BatchSize = defaultStudentOwnerBatchSize
	}
	if o.MaxPasses <= 0 {
		o.MaxPasses = defaultStudentOwnerMaxPasses
	}
	if o.MaxAttempts <= 0 {
		o.MaxAttempts = defaultStudentOwnerMaxAttempts
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	return o
}

// StudentOwnerBackfillCheckpoint is the persisted per-tenant state and runtime
// evidence of the backfill. Counters are cumulative across runs and passes;
// Pass, HighWaterID and PassWrites describe the pass in progress.
type StudentOwnerBackfillCheckpoint struct {
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
	// GuardianMismatchCount counts students whose legacy guardian name,
	// contact, e-mail or phone value has no counterpart among the guardian
	// profiles and phone numbers linked to that child. Those values have no
	// target column; reporting them is what keeps them from being dropped.
	GuardianMismatchCount int64 `bun:"guardian_mismatch_count" json:"guardian_mismatch_count"`
	// CareStateMismatchCount counts students whose legacy sick/excused flag is
	// raised without an equivalent open day in active.student_status_days, the
	// authority the targets rely on. Those flags have no target column either.
	CareStateMismatchCount int64 `bun:"care_state_mismatch_count" json:"care_state_mismatch_count"`
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
// checksums, no mismatched or orphaned rows, every legacy guardian value
// reconciled and the effective care state equivalent.
func (c StudentOwnerBackfillCheckpoint) Verified() bool {
	return c.VerifiedAt != nil && c.MismatchCount == 0 &&
		c.GuardianMismatchCount == 0 && c.CareStateMismatchCount == 0 &&
		c.SourceCount == c.TargetCount && c.SourceChecksum == c.TargetChecksum
}

// OldestUnmigratedAge is the age of the oldest source change not yet
// reflected in the targets at the last verification, or zero.
func (c StudentOwnerBackfillCheckpoint) OldestUnmigratedAge(now time.Time) time.Duration {
	if c.OldestUnmigratedAt == nil {
		return 0
	}
	return now.Sub(*c.OldestUnmigratedAt)
}

// StudentOwnerBackfillReport is the per-tenant state after a run or status read.
type StudentOwnerBackfillReport struct {
	Tenants []StudentOwnerBackfillCheckpoint `json:"tenants"`
	// MissingTenants lists schools without any checkpoint: they have never
	// been visited, for example after a reset or when a school was created
	// after the last run.
	MissingTenants []int64 `json:"missing_tenants"`
}

// Stable reports whether every school has a verified, stable checkpoint. This
// is the Cutover precondition; it says nothing about writes after StableAt.
func (r *StudentOwnerBackfillReport) Stable() bool {
	return r != nil && len(r.Unstable()) == 0
}

// Unstable returns the schools that still block Cutover: unvisited ones and
// those whose last pass changed rows or failed verification.
func (r *StudentOwnerBackfillReport) Unstable() []int64 {
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

type studentOwnerBatch struct {
	Scanned  int64
	LastID   int64
	Rejected int64
	Copied   int64
	Skipped  int64
	Attempts int
	Duration time.Duration
}

// studentOwnerEligibleBatch selects one keyset batch of source rows. Rows whose
// person belongs to another tenant are excluded: users.students still carries
// the single-column person FK, while users.student_profiles references
// users.persons(tenant_id, id), and the superuser connection would otherwise
// launder such a row into People storage. They count as rejected and keep the
// tenant unverified. group_id needs no such filter — the old table already has
// the composite education.groups(tenant_id, id) foreign key.
const studentOwnerEligibleBatch = `
	batch AS (
		SELECT s.id, s.tenant_id, s.person_id, s.created_at, s.updated_at,
		       s.address_street, s.address_city, s.address_postal_code, s.extra_info,
		       s.photo_path, s.photo_consent_given_at, s.photo_consent_given_by,
		       s.agb_accepted_at, s.data_processing_accepted_at, s.email_contact_accepted_at,
		       s.school_class, s.group_id, s.status, s.enrolled_from, s.enrolled_until,
		       s.supervisor_notes, s.health_info, s.pickup_status, s.departure_days,
		       s.allowed_departure_modes, s.departure_companion_note, s.pickup_days, s.bus_days
		FROM users.students AS s
		WHERE s.tenant_id = ? AND s.id > ?
		ORDER BY s.id
		LIMIT ?
	),
	eligible AS (
		SELECT b.*
		FROM batch AS b
		JOIN users.persons AS p ON p.id = b.person_id AND p.tenant_id = b.tenant_id
	)`

// studentOwnerCopyBatch copies one batch into all three targets and returns the
// batch statistics. Student identity is preserved throughout the split: the
// People profile, its School Membership and the membership's Care Plan all keep
// the users.students id, so every foreign key that names a student today points
// at the same number after Cutover. All three upserts only touch rows whose
// content differs, so reruns of a completed batch are no-ops and count as
// skipped.
//
// The membership's deleted_at is written as NULL on purpose. users.students has
// no soft deletion, so while it stays authoritative an enrollment that the old
// table still holds is not retired, and a stray retirement in the target would
// otherwise never converge.
const studentOwnerCopyBatch = `
	WITH ` + studentOwnerEligibleBatch + `,
	profiles AS (
		INSERT INTO users.student_profiles AS p
			(id, tenant_id, person_id, address_street, address_city, address_postal_code,
			 extra_info, photo_path, photo_consent_given_at, photo_consent_given_by,
			 agb_accepted_at, data_processing_accepted_at, email_contact_accepted_at,
			 created_at, updated_at)
		SELECT id, tenant_id, person_id, address_street, address_city, address_postal_code,
		       extra_info, photo_path, photo_consent_given_at, photo_consent_given_by,
		       agb_accepted_at, data_processing_accepted_at, email_contact_accepted_at,
		       created_at, updated_at
		FROM eligible
		ON CONFLICT (id) DO UPDATE SET
			tenant_id = EXCLUDED.tenant_id,
			person_id = EXCLUDED.person_id,
			address_street = EXCLUDED.address_street,
			address_city = EXCLUDED.address_city,
			address_postal_code = EXCLUDED.address_postal_code,
			extra_info = EXCLUDED.extra_info,
			photo_path = EXCLUDED.photo_path,
			photo_consent_given_at = EXCLUDED.photo_consent_given_at,
			photo_consent_given_by = EXCLUDED.photo_consent_given_by,
			agb_accepted_at = EXCLUDED.agb_accepted_at,
			data_processing_accepted_at = EXCLUDED.data_processing_accepted_at,
			email_contact_accepted_at = EXCLUDED.email_contact_accepted_at,
			created_at = EXCLUDED.created_at,
			updated_at = EXCLUDED.updated_at
		WHERE (p.tenant_id, p.person_id, p.address_street, p.address_city, p.address_postal_code,
		       p.extra_info, p.photo_path, p.photo_consent_given_at, p.photo_consent_given_by,
		       p.agb_accepted_at, p.data_processing_accepted_at, p.email_contact_accepted_at,
		       p.created_at, p.updated_at)
			IS DISTINCT FROM
			(EXCLUDED.tenant_id, EXCLUDED.person_id, EXCLUDED.address_street, EXCLUDED.address_city,
			 EXCLUDED.address_postal_code, EXCLUDED.extra_info, EXCLUDED.photo_path,
			 EXCLUDED.photo_consent_given_at, EXCLUDED.photo_consent_given_by,
			 EXCLUDED.agb_accepted_at, EXCLUDED.data_processing_accepted_at,
			 EXCLUDED.email_contact_accepted_at, EXCLUDED.created_at, EXCLUDED.updated_at)
		RETURNING p.id
	),
	memberships AS (
		INSERT INTO users.student_school_memberships AS m
			(id, tenant_id, student_profile_id, school_class, group_id, status,
			 enrolled_from, enrolled_until, created_at, updated_at, deleted_at)
		SELECT id, tenant_id, id, school_class, group_id, status,
		       enrolled_from, enrolled_until, created_at, updated_at, NULL::timestamptz
		FROM eligible
		ON CONFLICT (id) DO UPDATE SET
			tenant_id = EXCLUDED.tenant_id,
			student_profile_id = EXCLUDED.student_profile_id,
			school_class = EXCLUDED.school_class,
			group_id = EXCLUDED.group_id,
			status = EXCLUDED.status,
			enrolled_from = EXCLUDED.enrolled_from,
			enrolled_until = EXCLUDED.enrolled_until,
			created_at = EXCLUDED.created_at,
			updated_at = EXCLUDED.updated_at,
			deleted_at = EXCLUDED.deleted_at
		WHERE (m.tenant_id, m.student_profile_id, m.school_class, m.group_id, m.status,
		       m.enrolled_from, m.enrolled_until, m.created_at, m.updated_at, m.deleted_at)
			IS DISTINCT FROM
			(EXCLUDED.tenant_id, EXCLUDED.student_profile_id, EXCLUDED.school_class, EXCLUDED.group_id,
			 EXCLUDED.status, EXCLUDED.enrolled_from, EXCLUDED.enrolled_until, EXCLUDED.created_at,
			 EXCLUDED.updated_at, EXCLUDED.deleted_at)
		RETURNING m.id
	),
	care AS (
		INSERT INTO users.student_care_profiles AS c
			(membership_id, tenant_id, supervisor_notes, health_info, pickup_status,
			 departure_days, allowed_departure_modes, departure_companion_note,
			 pickup_days, bus_days, created_at, updated_at)
		SELECT id, tenant_id, supervisor_notes, health_info, pickup_status,
		       departure_days, allowed_departure_modes, departure_companion_note,
		       pickup_days, bus_days, created_at, updated_at
		FROM eligible
		ON CONFLICT (membership_id) DO UPDATE SET
			tenant_id = EXCLUDED.tenant_id,
			supervisor_notes = EXCLUDED.supervisor_notes,
			health_info = EXCLUDED.health_info,
			pickup_status = EXCLUDED.pickup_status,
			departure_days = EXCLUDED.departure_days,
			allowed_departure_modes = EXCLUDED.allowed_departure_modes,
			departure_companion_note = EXCLUDED.departure_companion_note,
			pickup_days = EXCLUDED.pickup_days,
			bus_days = EXCLUDED.bus_days,
			created_at = EXCLUDED.created_at,
			updated_at = EXCLUDED.updated_at
		WHERE (c.tenant_id, c.supervisor_notes, c.health_info, c.pickup_status,
		       c.departure_days, c.allowed_departure_modes, c.departure_companion_note,
		       c.pickup_days, c.bus_days, c.created_at, c.updated_at)
			IS DISTINCT FROM
			(EXCLUDED.tenant_id, EXCLUDED.supervisor_notes, EXCLUDED.health_info, EXCLUDED.pickup_status,
			 EXCLUDED.departure_days, EXCLUDED.allowed_departure_modes, EXCLUDED.departure_companion_note,
			 EXCLUDED.pickup_days, EXCLUDED.bus_days, EXCLUDED.created_at, EXCLUDED.updated_at)
		RETURNING c.membership_id AS id
	)
	SELECT (SELECT count(*) FROM batch) AS scanned,
	       (SELECT coalesce(max(id), ?) FROM batch) AS last_id,
	       (SELECT count(*) FROM batch) - (SELECT count(*) FROM eligible) AS rejected,
	       (SELECT count(*) FROM (SELECT id FROM profiles UNION SELECT id FROM memberships
	                              UNION SELECT id FROM care) AS written) AS copied`

// RunStudentOwnerBackfill copies users.students into users.student_profiles,
// users.student_school_memberships and users.student_care_profiles for every
// school, in deterministic tenant/id batches with a persisted high-water mark.
// Each batch commits on its own and is idempotent; interrupting the run and
// calling it again resumes at the checkpoint. After each full pass the tenant is
// verified by count, canonical checksum, row-wise mismatch, legacy guardian
// reconciliation and effective care state. A tenant becomes stable once a pass
// changes nothing and every verification matches. The old table is never
// modified.
func RunStudentOwnerBackfill(ctx context.Context, db *bun.DB, opts StudentOwnerBackfillOptions) (*StudentOwnerBackfillReport, error) {
	if db == nil {
		return nil, errors.New("student owner backfill: database is required")
	}
	opts = opts.withDefaults()
	release, err := lockStorageBackfill(ctx, db, StudentOwnerBackfillName)
	if err != nil {
		return nil, err
	}
	defer release()
	if err := assertStudentSourceIsBaseTable(ctx, db); err != nil {
		return nil, err
	}
	var tenantIDs []int64
	if err := db.NewRaw(`SELECT id FROM platform.schools ORDER BY id`).Scan(ctx, &tenantIDs); err != nil {
		return nil, fmt.Errorf("student owner backfill: list schools: %w", err)
	}
	// A failing school must not leave the remaining schools unvisited: each
	// keeps its own checkpoint, so the others proceed and the failures are
	// reported together. Cancellation stops the run immediately.
	var failures []error
	for _, tenantID := range tenantIDs {
		if err := runStudentOwnerTenant(ctx, db, opts, tenantID); err != nil {
			if ctx.Err() != nil {
				return nil, err
			}
			opts.Logger.Error("student owner backfill tenant failed",
				"tenant_id", tenantID,
				"error", err)
			failures = append(failures, err)
		}
	}
	// The two target sequences only have to stay above the preserved
	// identities; nothing allocates from them before Cutover, which owns the
	// hand-over from the users.students sequence.
	if _, err := db.ExecContext(ctx, `
		SELECT setval('users.student_profiles_id_seq', GREATEST(
			COALESCE((SELECT max(id) FROM users.student_profiles), 1),
			(SELECT last_value FROM users.student_profiles_id_seq),
			(SELECT last_value FROM users.students_id_seq)), true),
		       setval('users.student_school_memberships_id_seq', GREATEST(
			COALESCE((SELECT max(id) FROM users.student_school_memberships), 1),
			(SELECT last_value FROM users.student_school_memberships_id_seq),
			(SELECT last_value FROM users.students_id_seq)), true)`); err != nil {
		return nil, fmt.Errorf("student owner backfill: align target sequences: %w", err)
	}
	report, err := StudentOwnerBackfillStatus(ctx, db)
	if err != nil {
		return nil, err
	}
	opts.Logger.Info("student owner backfill finished",
		"tenants", len(report.Tenants),
		"stable", report.Stable(),
		"unstable_tenants", report.Unstable(),
		"failed_tenants", len(failures))
	return report, errors.Join(failures...)
}

type studentOwnerTenantRun struct {
	db        *bun.DB
	opts      StudentOwnerBackfillOptions
	tenantID  int64
	durations []time.Duration
	poolWait  time.Duration
}

func runStudentOwnerTenant(ctx context.Context, db *bun.DB, opts StudentOwnerBackfillOptions, tenantID int64) error {
	run := &studentOwnerTenantRun{db: db, opts: opts, tenantID: tenantID}
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
	opts.Logger.Warn("student owner backfill tenant not stable after pass limit",
		"tenant_id", tenantID,
		"pass", cp.Pass,
		"mismatch_count", cp.MismatchCount,
		"guardian_mismatch_count", cp.GuardianMismatchCount,
		"care_state_mismatch_count", cp.CareStateMismatchCount,
		"rows_rejected", cp.RowsRejected)
	return nil
}

func (r *studentOwnerTenantRun) loadCheckpoint(ctx context.Context) (*StudentOwnerBackfillCheckpoint, error) {
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO platform.storage_backfill_checkpoints (backfill, tenant_id)
		VALUES (?, ?) ON CONFLICT (backfill, tenant_id) DO NOTHING`, StudentOwnerBackfillName, r.tenantID); err != nil {
		return nil, fmt.Errorf("student owner backfill: init checkpoint for tenant %d: %w", r.tenantID, err)
	}
	cp := new(StudentOwnerBackfillCheckpoint)
	if err := r.db.NewSelect().Model(cp).
		Where("backfill = ? AND tenant_id = ?", StudentOwnerBackfillName, r.tenantID).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("student owner backfill: load checkpoint for tenant %d: %w", r.tenantID, err)
	}
	return cp, nil
}

func (r *studentOwnerTenantRun) copyPass(ctx context.Context, cp *StudentOwnerBackfillCheckpoint) error {
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
// zero because the target can only accept the new student row of a person after
// the orphaned profile of their deleted predecessor has been removed.
func (r *studentOwnerTenantRun) copyBatch(ctx context.Context, cp *StudentOwnerBackfillCheckpoint) (result studentOwnerBatch, resultErr error) {
	var pending studentOwnerRetries
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
			pending.lockTimeouts, float64(pending.lockWait)/float64(time.Millisecond), StudentOwnerBackfillName, r.tenantID)
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
			return studentOwnerBatch{}, fmt.Errorf("student owner backfill: tenant %d batch after id %d: %w", r.tenantID, cp.HighWaterID, err)
		}
		restart := code == sqlStateUniqueViolation
		if restart {
			// Persist the rewind so a crash before the next commit resumes
			// the restarted pass instead of the stale mark. Restarts have
			// their own budget: each one is caused by a source change, not
			// by contention on the same batch.
			restarts++
			if restarts > r.opts.MaxAttempts {
				return studentOwnerBatch{}, fmt.Errorf("student owner backfill: tenant %d restarted the pass %d times without converging: %w", r.tenantID, restarts-1, err)
			}
			if err := r.rewindPass(ctx, cp); err != nil {
				return studentOwnerBatch{}, err
			}
		} else {
			attempt++
			if attempt > r.opts.MaxAttempts {
				return studentOwnerBatch{}, fmt.Errorf("student owner backfill: tenant %d batch after id %d gave up after %d attempts: %w", r.tenantID, cp.HighWaterID, attempt-1, err)
			}
		}
		r.opts.Logger.Warn("student owner backfill batch retry",
			"tenant_id", r.tenantID,
			"attempt", attempt,
			"restarts", restarts,
			"sqlstate", code,
			"restart_pass", restart)
		select {
		case <-ctx.Done():
			return studentOwnerBatch{}, ctx.Err()
		case <-time.After(studentOwnerRetryBackoff * time.Duration(attempt+restarts)):
		}
		pending.retried++
	}
}

// studentOwnerRetries accumulates the transient failures of one batch until
// its successful commit persists them with the checkpoint.
type studentOwnerRetries struct {
	retried, deadlocks, serialization, lockTimeouts int64
	lockWait                                        time.Duration
}

// record classifies a failed attempt and reports whether it may be retried.
func (p *studentOwnerRetries) record(code string) bool {
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

// rewindPass releases the per-person unique key before retrying. Only one
// profile per (tenant, person) may exist, so a conflict means some target
// profile still claims a person its own source row no longer names — either
// because that source row was physically deleted and cannot be re-read at all,
// or because the person was reassigned to another child. Rewinding alone would
// meet the same claim on every restart, so the stale profile goes first and the
// restarted pass rebuilds it from the authoritative source.
func (r *studentOwnerTenantRun) rewindPass(ctx context.Context, cp *StudentOwnerBackfillCheckpoint) error {
	return r.reconcileOrphans(ctx, cp, true)
}

func (r *studentOwnerTenantRun) tryCopyBatch(ctx context.Context, cp *StudentOwnerBackfillCheckpoint, attempt int, pending *studentOwnerRetries) (studentOwnerBatch, error) {
	var batch studentOwnerBatch
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
			WHERE backfill = ? AND tenant_id = ? FOR UPDATE`, StudentOwnerBackfillName, r.tenantID); err != nil {
			return err
		}
		if err := tx.NewRaw(studentOwnerCopyBatch, r.tenantID, cp.HighWaterID, r.opts.BatchSize, cp.HighWaterID).
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
			float64(pending.lockWait)/float64(time.Millisecond), StudentOwnerBackfillName, r.tenantID); err != nil {
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
		return studentOwnerBatch{}, err
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

// removeOrphans deletes target profiles whose source row was physically
// deleted. Memberships and care profiles follow through the cascade.
func (r *studentOwnerTenantRun) removeOrphans(ctx context.Context, cp *StudentOwnerBackfillCheckpoint) error {
	return r.reconcileOrphans(ctx, cp, false)
}

// Cleanup and any conflict rewind commit together with their counters. The
// end-of-pass sweep only drops profiles whose source row is gone; a rewind also
// drops those whose source row now names a different person, because that stale
// claim on the per-person unique key is what rejected the batch. A changed
// person alone is no reason to delete: the ordinary copy updates it in place,
// and only a conflict proves the update could not run.
func (r *studentOwnerTenantRun) reconcileOrphans(ctx context.Context, cp *StudentOwnerBackfillCheckpoint, rewind bool) error {
	var removed int64
	err := r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `SET LOCAL lock_timeout = '5s'; SET LOCAL statement_timeout = '60s'`); err != nil {
			return err
		}
		if err := tx.NewRaw(`
			WITH removed AS (
				DELETE FROM users.student_profiles AS p
				WHERE p.tenant_id = ?
				  AND NOT EXISTS (
					SELECT 1 FROM users.students AS s
					WHERE s.id = p.id AND s.tenant_id = p.tenant_id
					  AND (NOT ? OR s.person_id = p.person_id))
				RETURNING p.id
			)
			SELECT count(*) FROM removed`, r.tenantID, rewind).Scan(ctx, &removed); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
			UPDATE platform.storage_backfill_checkpoints SET
				high_water_id = CASE WHEN ? THEN 0 ELSE high_water_id END,
				rows_removed = rows_removed + ?, pass_writes = pass_writes + ?, updated_at = now()
			WHERE backfill = ? AND tenant_id = ?`, rewind, removed, removed, StudentOwnerBackfillName, r.tenantID)
		return err
	})
	if err != nil {
		return fmt.Errorf("student owner backfill: tenant %d remove orphans: %w", r.tenantID, err)
	}
	cp.RowsRemoved += removed
	cp.PassWrites += removed
	if rewind {
		cp.HighWaterID = 0
	}
	return nil
}

func (r *studentOwnerTenantRun) startPass(ctx context.Context, cp *StudentOwnerBackfillCheckpoint) error {
	cp.Pass++
	cp.HighWaterID = 0
	cp.PassWrites = 0
	cp.PassCompleted = false
	cp.Stable = false
	if _, err := r.db.ExecContext(ctx, `
		UPDATE platform.storage_backfill_checkpoints SET
			pass = ?, high_water_id = 0, pass_writes = 0, pass_completed = false, stable = false, updated_at = now()
		WHERE backfill = ? AND tenant_id = ?`, cp.Pass, StudentOwnerBackfillName, r.tenantID); err != nil {
		return fmt.Errorf("student owner backfill: tenant %d start pass %d: %w", r.tenantID, cp.Pass, err)
	}
	return nil
}

func (r *studentOwnerTenantRun) batchPercentiles() (p95, maxMs int64) {
	if len(r.durations) == 0 {
		return 0, 0
	}
	sorted := slices.Clone(r.durations)
	slices.Sort(sorted)
	index := max((len(sorted)*95+99)/100, 1)
	return sorted[index-1].Milliseconds(), sorted[len(sorted)-1].Milliseconds()
}

// StudentOwnerBackfillStatus reads every tenant checkpoint without changing
// anything. Schools without a checkpoint have not been visited yet.
func StudentOwnerBackfillStatus(ctx context.Context, db *bun.DB) (*StudentOwnerBackfillReport, error) {
	if db == nil {
		return nil, errors.New("student owner backfill: database is required")
	}
	report := &StudentOwnerBackfillReport{Tenants: []StudentOwnerBackfillCheckpoint{}, MissingTenants: []int64{}}
	if err := db.NewSelect().Model(&report.Tenants).
		Where("backfill = ?", StudentOwnerBackfillName).
		OrderExpr("tenant_id").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("student owner backfill: read checkpoints: %w", err)
	}
	if err := db.NewRaw(`
		SELECT s.id FROM platform.schools AS s
		WHERE NOT EXISTS (
			SELECT 1 FROM platform.storage_backfill_checkpoints AS c
			WHERE c.backfill = ? AND c.tenant_id = s.id
		)
		ORDER BY s.id`, StudentOwnerBackfillName).Scan(ctx, &report.MissingTenants); err != nil {
		return nil, fmt.Errorf("student owner backfill: list unvisited schools: %w", err)
	}
	return report, nil
}

// ResetStudentOwnerBackfill discards every target row and checkpoint so the
// backfill restarts from zero. It refuses once users.students is no longer the
// authoritative base table, because after Cutover the targets hold live data.
func ResetStudentOwnerBackfill(ctx context.Context, db *bun.DB) error {
	if db == nil {
		return errors.New("student owner backfill: database is required")
	}
	release, err := lockStorageBackfill(ctx, db, StudentOwnerBackfillName)
	if err != nil {
		return err
	}
	defer release()
	if err := assertStudentSourceIsBaseTable(ctx, db); err != nil {
		return err
	}
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			TRUNCATE users.student_care_profiles, users.student_school_memberships, users.student_profiles;
			DELETE FROM platform.storage_backfill_checkpoints WHERE backfill = ?;`, StudentOwnerBackfillName); err != nil {
			return fmt.Errorf("student owner backfill: reset targets: %w", err)
		}
		return nil
	})
}

// assertStudentSourceIsBaseTable guards every target-only write: Cutover
// replaces users.students with a compatibility view, after which the targets
// are authoritative and must not be overwritten from the old shape.
func assertStudentSourceIsBaseTable(ctx context.Context, db bun.IDB) error {
	var kind string
	if err := db.NewRaw(`SELECT relkind::text FROM pg_class WHERE oid = 'users.students'::regclass`).Scan(ctx, &kind); err != nil {
		return fmt.Errorf("student owner backfill: inspect users.students: %w", err)
	}
	if kind != "r" {
		return fmt.Errorf("student owner backfill: users.students is not a base table (relkind %q); the targets are authoritative after Cutover", kind)
	}
	return nil
}
