package migrations

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

// studentOwnerCutoverTimeout bounds the whole switch. The write lock it holds
// stops every student write in every school, so an unexpectedly slow final
// delta has to fail and be retried rather than extend the outage.
const studentOwnerCutoverTimeout = 60 * time.Second

// finalizeStudentOwnerStorage applies the last backfill delta, proves the three
// targets reproduce users.students for every school, and switches the schema —
// all inside one transaction that holds the students write lock.
//
// It takes the backfill's own advisory lock first, so a resumable run, its
// reset and this switch can never interleave: the final delta must be the last
// copy, and nothing may move the high-water mark behind it.
//
// switchSchema is the schema change itself. It is a parameter so the cutover
// test can drive the same lock, delta and verification against a failing or
// partial switch and prove the transaction rolls all of it back together.
func finalizeStudentOwnerStorage(ctx context.Context, db *bun.DB, switchSchema func(context.Context, bun.Tx) error) error {
	if db == nil {
		return errors.New("student owner cutover: database is required")
	}
	release, err := lockStorageBackfill(ctx, db, StudentOwnerBackfillName)
	if err != nil {
		return err
	}
	defer release()
	// Running twice must not archive the archive. studentOwnerCutoverUp
	// resumes VALIDATE CONSTRAINT when the view is already in place; this
	// helper is only the switch itself and still refuses.
	if err := assertStudentSourceIsBaseTable(ctx, db); err != nil {
		return fmt.Errorf("student owner cutover: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, studentOwnerCutoverTimeout)
	defer cancel()
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			SET LOCAL lock_timeout = '5s';
			SET LOCAL statement_timeout = '60s';
			LOCK TABLE users.students, users.student_profiles,
				users.student_school_memberships, users.student_care_profiles
				IN ACCESS EXCLUSIVE MODE`); err != nil {
			return fmt.Errorf("student owner cutover: lock student storage: %w", err)
		}
		var tenantIDs []int64
		if err := tx.NewRaw(`SELECT id FROM platform.schools ORDER BY id`).Scan(ctx, &tenantIDs); err != nil {
			return fmt.Errorf("student owner cutover: list schools: %w", err)
		}
		for _, tenantID := range tenantIDs {
			if err := reconcileStudentOwnerFinalDelta(ctx, tx, tenantID); err != nil {
				return err
			}
		}
		if err := alignStudentOwnerSequences(ctx, tx); err != nil {
			return err
		}
		return switchSchema(ctx, tx)
	})
}

func studentOwnerRelationKind(ctx context.Context, db bun.IDB) (string, error) {
	var kind string
	if err := db.NewRaw(`SELECT relkind::text FROM pg_class WHERE oid = 'users.students'::regclass`).Scan(ctx, &kind); err != nil {
		return "", fmt.Errorf("student owner cutover: inspect users.students: %w", err)
	}
	return kind, nil
}

// reconcileStudentOwnerFinalDelta copies everything the resumable backfill has
// not seen yet for one school, drops the targets of source rows deleted since,
// and then holds the two shapes against each other. Unlike a backfill pass this
// is not keyset-batched: the write lock is already held, the delta is the small
// remainder, and a partial batch would defeat the point of verifying inside the
// same transaction that switches the schema.
func reconcileStudentOwnerFinalDelta(ctx context.Context, tx bun.Tx, tenantID int64) error {
	var checkpointed bool
	if err := tx.NewRaw(`SELECT EXISTS (
		SELECT 1 FROM platform.storage_backfill_checkpoints
		WHERE backfill = ?0 AND tenant_id = ?1 AND pass_completed
	) OR NOT EXISTS (SELECT 1 FROM users.students WHERE tenant_id = ?1)`,
		StudentOwnerBackfillName, tenantID).Scan(ctx, &checkpointed); err != nil {
		return fmt.Errorf("student owner cutover: tenant %d checkpoint: %w", tenantID, err)
	}
	if !checkpointed {
		return fmt.Errorf("student owner cutover: tenant %d requires a completed backfill pass; run `phoenix backfill student-owner` first", tenantID)
	}
	delta, err := applyStudentOwnerFinalDelta(ctx, tx, tenantID)
	if err != nil {
		return err
	}
	verification, err := verifyStudentOwnerTenant(ctx, tx, tenantID)
	if err != nil {
		return err
	}
	if !verification.Equal() {
		return fmt.Errorf("student owner cutover: tenant %d is not reproducible: %s", tenantID, verification.Describe())
	}
	return persistStudentOwnerCutoverEvidence(ctx, tx, tenantID, verification, delta.Copied, delta.Removed)
}

// persistStudentOwnerCutoverEvidence leaves the switch's own verdict in the
// checkpoint the backfill wrote. The row is what an operator reads during the
// rollback window, so the final delta must be visible in it and not only in the
// migration log.
func persistStudentOwnerCutoverEvidence(ctx context.Context, tx bun.Tx, tenantID int64, verification StudentOwnerVerification, copied, removed int64) error {
	if _, err := tx.NewRaw(`INSERT INTO platform.storage_backfill_checkpoints (backfill, tenant_id)
		VALUES (?, ?) ON CONFLICT (backfill, tenant_id) DO NOTHING`, StudentOwnerBackfillName, tenantID).Exec(ctx); err != nil {
		return fmt.Errorf("student owner cutover: tenant %d init checkpoint: %w", tenantID, err)
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE platform.storage_backfill_checkpoints SET
			rows_copied = rows_copied + ?, rows_removed = rows_removed + ?,
			batches_completed = batches_completed + 1,
			source_count = ?, target_count = ?, source_checksum = ?, target_checksum = ?,
			mismatch_count = ?, guardian_mismatch_count = ?, care_state_mismatch_count = ?,
			oldest_unmigrated_at = NULL, pass_writes = 0, pass_completed = true,
			stable = true, stable_at = now(), verified_at = now(), updated_at = now()
		WHERE backfill = ? AND tenant_id = ?`,
		copied, removed,
		verification.SourceCount, verification.TargetCount, verification.SourceChecksum, verification.TargetChecksum,
		verification.MismatchCount, verification.GuardianMismatchCount, verification.CareStateMismatchCount,
		StudentOwnerBackfillName, tenantID)
	if err != nil {
		return fmt.Errorf("student owner cutover: tenant %d persist verification: %w", tenantID, err)
	}
	return nil
}

// alignStudentOwnerSequences hands the identity allocation over from the old
// table. Student ids stay the numbers every foreign key already names, so the
// profile sequence has to continue above the last one users.students issued.
// The membership sequence follows it: after the switch a new enrollment is
// allocated there, and it must never collide with an id the split preserved.
func alignStudentOwnerSequences(ctx context.Context, tx bun.Tx) error {
	if _, err := tx.ExecContext(ctx, `
		SELECT setval('users.student_profiles_id_seq', GREATEST(
			COALESCE((SELECT max(id) FROM users.student_profiles), 1),
			(SELECT last_value FROM users.student_profiles_id_seq),
			(SELECT last_value FROM users.students_id_seq)), true),
		       setval('users.student_school_memberships_id_seq', GREATEST(
			COALESCE((SELECT max(id) FROM users.student_school_memberships), 1),
			(SELECT last_value FROM users.student_school_memberships_id_seq),
			(SELECT last_value FROM users.students_id_seq)), true)`); err != nil {
		return fmt.Errorf("student owner cutover: align target sequences: %w", err)
	}
	return nil
}
