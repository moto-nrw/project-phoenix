package migrations

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/uptrace/bun"
)

// staffOwnerCutoverTimeout bounds the whole switch. The write lock it holds
// stops every staff write in every school, so an unexpectedly slow final
// delta has to fail and be retried rather than extend the outage.
const staffOwnerCutoverTimeout = 60 * time.Second

// StaffOwnerVerification is the per-tenant verdict the switch requires: the
// joined targets must reproduce users.staff row for row.
type StaffOwnerVerification struct {
	SourceCount    int64
	TargetCount    int64
	SourceChecksum string
	TargetChecksum string
	MismatchCount  int64
}

// Equal reports whether counts, canonical checksums and the row-wise
// comparison all agree.
func (v StaffOwnerVerification) Equal() bool {
	return v.SourceCount == v.TargetCount && v.SourceChecksum == v.TargetChecksum && v.MismatchCount == 0
}

// Describe names every failing verdict for the migration error.
func (v StaffOwnerVerification) Describe() string {
	return fmt.Sprintf("source %d rows (%s), target %d rows (%s), %d mismatched rows",
		v.SourceCount, v.SourceChecksum, v.TargetCount, v.TargetChecksum, v.MismatchCount)
}

// finalizeStaffOwnerStorage applies the last backfill delta, proves the two
// targets reproduce users.staff for every school, and switches the schema,
// all inside one transaction that holds the staff write lock.
//
// It takes the backfill's own advisory lock first, so a resumable run, its
// reset and this switch can never interleave: the final delta must be the last
// copy, and nothing may move the high-water mark behind it.
//
// switchSchema is the schema change itself. It is a parameter so the cutover
// test can drive the same lock, delta and verification against a failing or
// partial switch and prove the transaction rolls all of it back together.
func finalizeStaffOwnerStorage(ctx context.Context, db *bun.DB, switchSchema func(context.Context, bun.Tx) error) error {
	if db == nil {
		return errors.New("staff owner cutover: database is required")
	}
	release, err := lockStorageBackfill(ctx, db, StaffOwnerBackfillName)
	if err != nil {
		return err
	}
	defer release()
	// Running twice must not archive the archive. staffOwnerCutoverUp resumes
	// VALIDATE CONSTRAINT when the view is already in place; this helper is
	// only the switch itself and still refuses.
	if err := assertStaffSourceIsBaseTable(ctx, db); err != nil {
		return fmt.Errorf("staff owner cutover: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, staffOwnerCutoverTimeout)
	defer cancel()
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			SET LOCAL lock_timeout = '5s';
			SET LOCAL statement_timeout = '60s';
			LOCK TABLE users.staff, users.staff_school_memberships, users.staff_employment_profiles
				IN ACCESS EXCLUSIVE MODE`); err != nil {
			return fmt.Errorf("staff owner cutover: lock staff storage: %w", err)
		}
		var tenantIDs []int64
		if err := tx.NewRaw(`SELECT id FROM platform.schools ORDER BY id`).Scan(ctx, &tenantIDs); err != nil {
			return fmt.Errorf("staff owner cutover: list schools: %w", err)
		}
		for _, tenantID := range tenantIDs {
			if err := reconcileStaffOwnerFinalDelta(ctx, tx, tenantID); err != nil {
				return err
			}
		}
		if err := alignStaffOwnerSequence(ctx, tx); err != nil {
			return err
		}
		return switchSchema(ctx, tx)
	})
}

func staffOwnerRelationKind(ctx context.Context, db bun.IDB) (string, error) {
	var kind string
	if err := db.NewRaw(`SELECT relkind::text FROM pg_class WHERE oid = 'users.staff'::regclass`).Scan(ctx, &kind); err != nil {
		return "", fmt.Errorf("staff owner cutover: inspect users.staff: %w", err)
	}
	return kind, nil
}

// reconcileStaffOwnerFinalDelta copies everything the resumable backfill has
// not seen yet for one school, drops the targets of source rows deleted since,
// and then holds the two shapes against each other. Unlike a backfill pass it
// is not keyset-batched: the write lock is already held, the delta is the small
// remainder, and one statement retires a rejoined person's old membership
// before it inserts the new one.
func reconcileStaffOwnerFinalDelta(ctx context.Context, tx bun.Tx, tenantID int64) error {
	var checkpointed bool
	if err := tx.NewRaw(`SELECT EXISTS (
		SELECT 1 FROM platform.storage_backfill_checkpoints
		WHERE backfill = ?0 AND tenant_id = ?1 AND pass_completed
	) OR NOT EXISTS (SELECT 1 FROM users.staff WHERE tenant_id = ?1)`,
		StaffOwnerBackfillName, tenantID).Scan(ctx, &checkpointed); err != nil {
		return fmt.Errorf("staff owner cutover: tenant %d checkpoint: %w", tenantID, err)
	}
	if !checkpointed {
		return fmt.Errorf("staff owner cutover: tenant %d requires a completed backfill pass; run `phoenix backfill staff-owner` first", tenantID)
	}
	var removed int64
	if err := tx.NewRaw(`
		WITH removed AS (
			DELETE FROM users.staff_school_memberships AS m
			WHERE m.tenant_id = ?
			  AND NOT EXISTS (SELECT 1 FROM users.staff AS s WHERE s.id = m.id AND s.tenant_id = m.tenant_id)
			RETURNING m.id
		)
		SELECT count(*) FROM removed`, tenantID).Scan(ctx, &removed); err != nil {
		return fmt.Errorf("staff owner cutover: tenant %d remove orphans: %w", tenantID, err)
	}
	var batch staffOwnerBatch
	if err := tx.NewRaw(staffOwnerCopyBatch, tenantID, 0, math.MaxInt32, 0).
		Scan(ctx, &batch.Scanned, &batch.LastID, &batch.Rejected, &batch.Copied); err != nil {
		return fmt.Errorf("staff owner cutover: tenant %d final delta: %w", tenantID, err)
	}
	verification, err := verifyStaffOwnerTenant(ctx, tx, tenantID)
	if err != nil {
		return err
	}
	if !verification.Equal() {
		return fmt.Errorf("staff owner cutover: tenant %d is not reproducible (%d rows reference another school's work-time model): %s",
			tenantID, batch.Rejected, verification.Describe())
	}
	return persistStaffOwnerCutoverEvidence(ctx, tx, tenantID, verification, batch.Copied, removed)
}

// verifyStaffOwnerTenant uses the backfill's own projections: one definition
// of equality, not a second one that could drift from it.
func verifyStaffOwnerTenant(ctx context.Context, tx bun.Tx, tenantID int64) (StaffOwnerVerification, error) {
	var v StaffOwnerVerification
	if err := tx.NewRaw(staffOwnerSourceChecksum, tenantID).Scan(ctx, &v.SourceCount, &v.SourceChecksum); err != nil {
		return v, fmt.Errorf("staff owner cutover: tenant %d source checksum: %w", tenantID, err)
	}
	if err := tx.NewRaw(staffOwnerTargetChecksum, tenantID).Scan(ctx, &v.TargetCount, &v.TargetChecksum); err != nil {
		return v, fmt.Errorf("staff owner cutover: tenant %d target checksum: %w", tenantID, err)
	}
	var oldest *time.Time
	if err := tx.NewRaw(staffOwnerMismatch, tenantID, tenantID).Scan(ctx, &v.MismatchCount, &oldest); err != nil {
		return v, fmt.Errorf("staff owner cutover: tenant %d mismatches: %w", tenantID, err)
	}
	return v, nil
}

// persistStaffOwnerCutoverEvidence leaves the switch's own verdict in the
// checkpoint the backfill wrote. The row is what an operator reads during the
// rollback window, so the final delta must be visible in it and not only in
// the migration log.
func persistStaffOwnerCutoverEvidence(ctx context.Context, tx bun.Tx, tenantID int64, verification StaffOwnerVerification, copied, removed int64) error {
	if _, err := tx.NewRaw(`INSERT INTO platform.storage_backfill_checkpoints (backfill, tenant_id)
		VALUES (?, ?) ON CONFLICT (backfill, tenant_id) DO NOTHING`, StaffOwnerBackfillName, tenantID).Exec(ctx); err != nil {
		return fmt.Errorf("staff owner cutover: tenant %d init checkpoint: %w", tenantID, err)
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE platform.storage_backfill_checkpoints SET
			rows_copied = rows_copied + ?, rows_removed = rows_removed + ?,
			batches_completed = batches_completed + 1,
			source_count = ?, target_count = ?, source_checksum = ?, target_checksum = ?,
			mismatch_count = ?, oldest_unmigrated_at = NULL, pass_writes = 0, pass_completed = true,
			stable = true, stable_at = now(), verified_at = now(), updated_at = now()
		WHERE backfill = ? AND tenant_id = ?`,
		copied, removed,
		verification.SourceCount, verification.TargetCount, verification.SourceChecksum, verification.TargetChecksum,
		verification.MismatchCount, StaffOwnerBackfillName, tenantID)
	if err != nil {
		return fmt.Errorf("staff owner cutover: tenant %d persist verification: %w", tenantID, err)
	}
	return nil
}

// alignStaffOwnerSequence hands identity allocation over from the old table.
// Staff ids stay the numbers every foreign key already names, so the
// membership sequence has to continue above the last one users.staff issued.
func alignStaffOwnerSequence(ctx context.Context, tx bun.Tx) error {
	if _, err := tx.ExecContext(ctx, `
		SELECT setval('users.staff_school_memberships_id_seq', GREATEST(
			COALESCE((SELECT max(id) FROM users.staff_school_memberships), 1),
			(SELECT last_value FROM users.staff_school_memberships_id_seq),
			(SELECT last_value FROM users.staff_id_seq)), true)`); err != nil {
		return fmt.Errorf("staff owner cutover: align membership sequence: %w", err)
	}
	return nil
}

// staffOwnerCutoverPrecondition answers, before the deployment stops the
// application, whether a staff row the switch can never copy exists. A row
// whose work-time model belongs to another school is rejected by every copy:
// more passes cannot close that gap, only a data correction can. Counts and
// checksums are mechanical and are closed by the switch's own final delta.
func staffOwnerCutoverPrecondition(ctx context.Context, db *bun.DB) error {
	if db == nil {
		return errors.New("staff owner cutover preflight: database is required")
	}
	var kind string
	if err := db.NewRaw(`SELECT coalesce((SELECT relkind::text FROM pg_class WHERE oid = to_regclass('users.staff')), '')`).
		Scan(ctx, &kind); err != nil {
		return fmt.Errorf("staff owner cutover preflight: inspect users.staff: %w", err)
	}
	if kind != "r" {
		return nil
	}
	var rejected []int64
	if err := db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `SET LOCAL lock_timeout = '5s'; SET LOCAL statement_timeout = '60s'`); err != nil {
			return err
		}
		return tx.NewRaw(`SELECT s.id FROM users.staff AS s
			JOIN config.work_time_models AS w ON w.id = s.work_time_model_id
			WHERE w.tenant_id <> s.tenant_id ORDER BY s.id`).Scan(ctx, &rejected)
	}); err != nil {
		return fmt.Errorf("staff owner cutover preflight: %w", err)
	}
	if len(rejected) > 0 {
		return fmt.Errorf("staff owner cutover preflight: staff rows %v reference another school's work-time model; correct them before the release", rejected)
	}
	return nil
}
