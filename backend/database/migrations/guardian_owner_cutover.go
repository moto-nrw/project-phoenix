package migrations

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

// guardianOwnerCutoverTimeout bounds the whole switch. The write lock it holds
// stops every guardian link write in every school, so an unexpectedly slow
// final delta has to fail and be retried rather than extend the outage.
const guardianOwnerCutoverTimeout = 60 * time.Second

// guardianOwnerFinalDeltaBatch is the keyset step of the final delta. The
// write lock is already held, so the batch only bounds one statement's
// working set; it is not a commit boundary.
const guardianOwnerFinalDeltaBatch = 5000

// GuardianOwnerVerification is the per-tenant verdict the switch requires: the
// joined targets must reproduce users.students_guardians row for row and every
// access row must carry the guardian's current account binding.
type GuardianOwnerVerification struct {
	SourceCount          int64
	TargetCount          int64
	SourceChecksum       string
	TargetChecksum       string
	MismatchCount        int64
	AccessMismatchCount  int64
	RejectedRows         int64
	RemovedTargetRows    int64
	CopiedOrUpdatedRows  int64
	VerificationSnapshot string
}

// Equal reports whether counts, canonical checksums, the row-wise comparison
// and the guardian access binding all agree.
func (v GuardianOwnerVerification) Equal() bool {
	return v.SourceCount == v.TargetCount && v.SourceChecksum == v.TargetChecksum &&
		v.MismatchCount == 0 && v.AccessMismatchCount == 0
}

// Describe names every failing verdict for the migration error.
func (v GuardianOwnerVerification) Describe() string {
	return fmt.Sprintf("source %d rows (%s), target %d rows (%s), %d mismatched rows, %d access mismatches, %d rejected rows",
		v.SourceCount, v.SourceChecksum, v.TargetCount, v.TargetChecksum, v.MismatchCount, v.AccessMismatchCount, v.RejectedRows)
}

// guardianCompatibilityInstalled reports whether the rollback-only mirror is
// in place: the switch has run when the routing trigger on the old table
// exists.
func guardianCompatibilityInstalled(ctx context.Context, db bun.IDB) (bool, error) {
	var installed bool
	if err := db.NewRaw(`SELECT EXISTS (
		SELECT 1 FROM pg_trigger WHERE tgname = 'students_guardians_route_compatibility'
		  AND tgrelid = 'users.students_guardians'::regclass AND NOT tgisinternal)`).Scan(ctx, &installed); err != nil {
		return false, fmt.Errorf("guardian owner cutover: inspect compatibility triggers: %w", err)
	}
	return installed, nil
}

// requireGuardianStorageBeforeCutover refuses the backfill, its reset and its
// rollback once the owner tables are authoritative: a reset would erase the
// owners' data and a batch would copy the mirror back onto itself.
func requireGuardianStorageBeforeCutover(ctx context.Context, db bun.IDB) error {
	installed, err := guardianCompatibilityInstalled(ctx, db)
	if err != nil {
		return err
	}
	if installed {
		return errors.New("guardian owner backfill: the storage is already cut over (#2756); users.student_guardian_relationships, users.student_guardian_pickup_permissions and auth.guardian_student_access are authoritative")
	}
	return nil
}

// finalizeGuardianOwnerStorage applies the last backfill delta, proves the
// three targets reproduce users.students_guardians for every school, and
// installs the compatibility shape, all inside one transaction that holds the
// guardian write lock.
//
// It takes the backfill's own advisory lock first, so a resumable run, its
// reset and this switch can never interleave: the final delta must be the last
// copy, and nothing may move the high-water mark behind it.
//
// switchSchema is the schema change itself. It is a parameter so the cutover
// tests can drive the same lock, delta and verification against a failing
// switch and prove the transaction rolls all of it back together.
func finalizeGuardianOwnerStorage(ctx context.Context, db *bun.DB, switchSchema func(context.Context, bun.Tx) error) error {
	if db == nil {
		return errors.New("guardian owner cutover: database is required")
	}
	release, err := lockStorageBackfill(ctx, db, GuardianOwnerBackfillName)
	if err != nil {
		return err
	}
	defer release()
	if err := requireGuardianStorageBeforeCutover(ctx, db); err != nil {
		return fmt.Errorf("guardian owner cutover: %w", err)
	}
	if err := assertGuardianSourceIsBaseTable(ctx, db); err != nil {
		return fmt.Errorf("guardian owner cutover: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, guardianOwnerCutoverTimeout)
	defer cancel()
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// users.guardian_profiles is locked too: the access rows mirror its
		// account binding, and a guardian linking an account between the
		// delta and the binding trigger would otherwise leave one behind.
		if _, err := tx.ExecContext(ctx, `
			SET LOCAL lock_timeout = '5s';
			SET LOCAL statement_timeout = '60s';
			SET LOCAL TIME ZONE 'UTC';
			LOCK TABLE users.students_guardians, users.student_guardian_relationships,
				users.student_guardian_pickup_permissions, auth.guardian_student_access,
				users.guardian_profiles
				IN ACCESS EXCLUSIVE MODE`); err != nil {
			return fmt.Errorf("guardian owner cutover: lock guardian storage: %w", err)
		}
		var tenantIDs []int64
		if err := tx.NewRaw(`SELECT id FROM platform.schools ORDER BY id`).Scan(ctx, &tenantIDs); err != nil {
			return fmt.Errorf("guardian owner cutover: list schools: %w", err)
		}
		for _, tenantID := range tenantIDs {
			if err := reconcileGuardianOwnerFinalDelta(ctx, tx, tenantID); err != nil {
				return err
			}
		}
		if err := alignGuardianOwnerSequence(ctx, tx); err != nil {
			return err
		}
		return switchSchema(ctx, tx)
	})
}

// reconcileGuardianOwnerFinalDelta copies everything the resumable backfill
// has not seen yet for one school, drops the targets of source rows deleted
// since, and then holds the two shapes against each other.
//
// Target relationships whose source row now names a different pair or carries
// a different primary/payer flag are dropped before the copy, as the backfill's
// conflict rewind does: they still claim a unique key their source row gave
// up, and the copy recreates them from the authoritative row with the source's
// timestamps.
func reconcileGuardianOwnerFinalDelta(ctx context.Context, tx bun.Tx, tenantID int64) error {
	var checkpointed bool
	if err := tx.NewRaw(`SELECT EXISTS (
		SELECT 1 FROM platform.storage_backfill_checkpoints
		WHERE backfill = ?0 AND tenant_id = ?1 AND pass_completed
	) OR NOT EXISTS (SELECT 1 FROM users.students_guardians WHERE tenant_id = ?1)`,
		GuardianOwnerBackfillName, tenantID).Scan(ctx, &checkpointed); err != nil {
		return fmt.Errorf("guardian owner cutover: tenant %d checkpoint: %w", tenantID, err)
	}
	if !checkpointed {
		return fmt.Errorf("guardian owner cutover: tenant %d requires a completed backfill pass; run `phoenix backfill guardian-owner` first", tenantID)
	}
	var verification GuardianOwnerVerification
	if err := tx.NewRaw(`
		WITH removed AS (
			DELETE FROM users.student_guardian_relationships AS r
			WHERE r.tenant_id = ?
			  AND NOT EXISTS (
				SELECT 1 FROM users.students_guardians AS sg
				WHERE sg.id = r.id AND sg.tenant_id = r.tenant_id
				  AND sg.student_id = r.student_id AND sg.guardian_profile_id = r.guardian_profile_id
				  AND sg.is_primary = r.is_primary AND sg.is_payer = r.is_payer)
			RETURNING r.id
		)
		SELECT count(*) FROM removed`, tenantID).Scan(ctx, &verification.RemovedTargetRows); err != nil {
		return fmt.Errorf("guardian owner cutover: tenant %d remove stale targets: %w", tenantID, err)
	}
	var highWater int64
	for {
		batch, err := copyGuardianOwnerRows(ctx, tx, tenantID, highWater, guardianOwnerFinalDeltaBatch)
		if err != nil {
			return fmt.Errorf("guardian owner cutover: tenant %d final delta after id %d: %w", tenantID, highWater, err)
		}
		verification.RejectedRows += batch.Rejected
		verification.CopiedOrUpdatedRows += batch.Copied
		highWater = batch.LastID
		if batch.Scanned < guardianOwnerFinalDeltaBatch {
			break
		}
	}
	if err := verifyGuardianOwnerCutover(ctx, tx, tenantID, &verification); err != nil {
		return err
	}
	if !verification.Equal() {
		return fmt.Errorf("guardian owner cutover: tenant %d is not reproducible: %s", tenantID, verification.Describe())
	}
	return persistGuardianOwnerCutoverEvidence(ctx, tx, tenantID, verification)
}

// verifyGuardianOwnerCutover reuses the backfill's own projections and
// checksums, so the switch proves exactly what the backfill promised, inside
// the transaction that applied the delta.
func verifyGuardianOwnerCutover(ctx context.Context, tx bun.Tx, tenantID int64, verification *GuardianOwnerVerification) error {
	shape, err := compareGuardianOwnerShapes(ctx, tx, tenantID)
	if err != nil {
		return fmt.Errorf("guardian owner cutover: tenant %d: %w", tenantID, err)
	}
	verification.SourceCount, verification.SourceChecksum = shape.SourceCount, shape.SourceChecksum
	verification.TargetCount, verification.TargetChecksum = shape.TargetCount, shape.TargetChecksum
	verification.MismatchCount, verification.AccessMismatchCount = shape.Mismatches, shape.AccessMismatches
	if err := tx.NewRaw(`SELECT pg_current_snapshot()::text`).Scan(ctx, &verification.VerificationSnapshot); err != nil {
		return fmt.Errorf("guardian owner cutover: tenant %d snapshot: %w", tenantID, err)
	}
	return nil
}

// persistGuardianOwnerCutoverEvidence leaves the switch's own verdict in the
// checkpoint the backfill wrote. The row is what an operator reads during the
// rollback window, so the final delta must be visible in it and not only in
// the migration log.
func persistGuardianOwnerCutoverEvidence(ctx context.Context, tx bun.Tx, tenantID int64, verification GuardianOwnerVerification) error {
	if _, err := tx.NewRaw(`INSERT INTO platform.storage_backfill_checkpoints (backfill, tenant_id)
		VALUES (?, ?) ON CONFLICT (backfill, tenant_id) DO NOTHING`, GuardianOwnerBackfillName, tenantID).Exec(ctx); err != nil {
		return fmt.Errorf("guardian owner cutover: tenant %d init checkpoint: %w", tenantID, err)
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE platform.storage_backfill_checkpoints SET
			rows_copied = rows_copied + ?, rows_removed = rows_removed + ?,
			batches_completed = batches_completed + 1,
			source_count = ?, target_count = ?, source_checksum = ?, target_checksum = ?,
			mismatch_count = ?, guardian_access_mismatch_count = ?, oldest_unmigrated_at = NULL,
			verification_snapshot = ?, high_water_id = 0, pass_writes = 0, pass_completed = true,
			stable = true, stable_at = now(), verified_at = now(), updated_at = now()
		WHERE backfill = ? AND tenant_id = ?`,
		verification.CopiedOrUpdatedRows, verification.RemovedTargetRows,
		verification.SourceCount, verification.TargetCount, verification.SourceChecksum, verification.TargetChecksum,
		verification.MismatchCount, verification.AccessMismatchCount, verification.VerificationSnapshot,
		GuardianOwnerBackfillName, tenantID)
	if err != nil {
		return fmt.Errorf("guardian owner cutover: tenant %d persist verification: %w", tenantID, err)
	}
	return nil
}

// alignGuardianOwnerSequence hands identity allocation over from the old
// table. Relationship ids stay the numbers the consent and meal-participation
// grants already name, so the relationship sequence has to continue above the
// last id users.students_guardians issued. The mirror's default follows in
// the compatibility install, so both images draw from one sequence.
func alignGuardianOwnerSequence(ctx context.Context, tx bun.Tx) error {
	if _, err := tx.ExecContext(ctx, `
		SELECT setval('users.student_guardian_relationships_id_seq', GREATEST(
			COALESCE((SELECT max(id) FROM users.student_guardian_relationships), 1),
			(SELECT last_value FROM users.student_guardian_relationships_id_seq),
			(SELECT last_value FROM users.students_guardians_id_seq)), true)`); err != nil {
		return fmt.Errorf("guardian owner cutover: align relationship sequence: %w", err)
	}
	return nil
}

// guardianOwnerCutoverPrecondition answers, before the deployment stops the
// application, whether a relationship the switch can never copy exists or a
// school has links but no completed backfill pass. Counts and checksums are
// mechanical and are closed by the switch's own final delta; a rejected row
// needs a data correction and a missing pass needs the backfill CLI.
//
// When the Backfill migration is pending in the same release, the pass does not
// exist yet and cannot: the CLI needs the checkpoint columns that migration
// adds. Its Up runs the pass, and the switch refuses without one, so only the
// rejected rows are asked then.
func guardianOwnerCutoverPrecondition(ctx context.Context, db *bun.DB) error {
	if db == nil {
		return errors.New("guardian owner cutover preflight: database is required")
	}
	installed, err := guardianCompatibilityInstalled(ctx, db)
	if err != nil {
		return err
	}
	if installed {
		return nil
	}
	var rejected, unvisited []int64
	if err := db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `SET LOCAL lock_timeout = '5s'; SET LOCAL statement_timeout = '60s'`); err != nil {
			return err
		}
		if err := tx.NewRaw(`SELECT sg.id FROM users.students_guardians AS sg
			LEFT JOIN users.guardian_profiles AS g ON g.id = sg.guardian_profile_id AND g.tenant_id = sg.tenant_id
			WHERE g.id IS NULL
			   OR (sg.is_primary AND EXISTS (
				SELECT 1 FROM users.students_guardians AS o
				WHERE o.tenant_id = sg.tenant_id AND o.student_id = sg.student_id AND o.is_primary AND o.id <> sg.id))
			ORDER BY sg.id`).Scan(ctx, &rejected); err != nil {
			return err
		}
		if migrationPending(ctx, guardianOwnerBackfillVersion) {
			return nil
		}
		return tx.NewRaw(`SELECT s.id FROM platform.schools AS s
			WHERE EXISTS (SELECT 1 FROM users.students_guardians AS sg WHERE sg.tenant_id = s.id)
			  AND NOT EXISTS (
				SELECT 1 FROM platform.storage_backfill_checkpoints AS c
				WHERE c.backfill = ? AND c.tenant_id = s.id AND c.pass_completed)
			ORDER BY s.id`, GuardianOwnerBackfillName).Scan(ctx, &unvisited)
	}); err != nil {
		return fmt.Errorf("guardian owner cutover preflight: %w", err)
	}
	if len(rejected) > 0 {
		return fmt.Errorf("guardian owner cutover preflight: users.students_guardians rows %v cannot be copied (guardian of another school or a second primary guardian of the child); correct them before the release", rejected)
	}
	if len(unvisited) > 0 {
		return fmt.Errorf("guardian owner cutover preflight: schools %v have no completed guardian owner backfill pass; run `phoenix backfill guardian-owner` before the release", unvisited)
	}
	return nil
}
