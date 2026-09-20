package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const studentOwnerBackfillVersion = "1.15.394"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentOwnerBackfillVersion,
		Description: "Backfill People, School Membership and Care Plan student storage from users.students in resumable tenant batches (#2758)",
		DependsOn:   []string{studentOwnerStorageExpandVersion, staffOwnerBackfillVersion},
	})
	Migrations.MustRegister(studentOwnerBackfillUp, studentOwnerBackfillDown)
}

// studentOwnerBackfillUp widens the shared checkpoint storage with this
// backfill's two extra verdicts and runs the initial copy. Every batch commits
// on its own, so an interrupted deployment simply resumes at the persisted
// high-water mark on the next run; that is why the DDL is idempotent, including
// for a checkpoint table that the Staff rollback dropped in between.
// users.students stays authoritative: no caller switches, no trigger or dual
// write is introduced, and the old rows are never modified. Remaining source
// writes are picked up by `phoenix backfill student-owner` before Cutover
// (#2759), which applies the final delta under its own lock.
func studentOwnerBackfillUp(ctx context.Context, db *bun.DB) error {
	err := db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS platform.storage_backfill_checkpoints (
				backfill TEXT NOT NULL,
				tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
				pass INTEGER NOT NULL DEFAULT 1,
				high_water_id BIGINT NOT NULL DEFAULT 0,
				pass_writes BIGINT NOT NULL DEFAULT 0,
				pass_completed BOOLEAN NOT NULL DEFAULT FALSE,
				stable BOOLEAN NOT NULL DEFAULT FALSE,
				rows_scanned BIGINT NOT NULL DEFAULT 0,
				rows_copied BIGINT NOT NULL DEFAULT 0,
				rows_skipped BIGINT NOT NULL DEFAULT 0,
				rows_rejected BIGINT NOT NULL DEFAULT 0,
				rows_removed BIGINT NOT NULL DEFAULT 0,
				batches_completed BIGINT NOT NULL DEFAULT 0,
				batches_retried BIGINT NOT NULL DEFAULT 0,
				deadlocks BIGINT NOT NULL DEFAULT 0,
				serialization_failures BIGINT NOT NULL DEFAULT 0,
				lock_timeouts BIGINT NOT NULL DEFAULT 0,
				source_count BIGINT NOT NULL DEFAULT 0,
				target_count BIGINT NOT NULL DEFAULT 0,
				source_checksum TEXT NOT NULL DEFAULT '',
				target_checksum TEXT NOT NULL DEFAULT '',
				mismatch_count BIGINT NOT NULL DEFAULT 0,
				oldest_unmigrated_at TIMESTAMPTZ,
				batch_p95_ms BIGINT NOT NULL DEFAULT 0,
				batch_max_ms BIGINT NOT NULL DEFAULT 0,
				pool_wait_ms BIGINT NOT NULL DEFAULT 0,
				lock_wait_ms DOUBLE PRECISION NOT NULL DEFAULT 0,
				verification_snapshot TEXT NOT NULL DEFAULT '',
				verified_at TIMESTAMPTZ,
				stable_at TIMESTAMPTZ,
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				PRIMARY KEY (backfill, tenant_id)
			);
			COMMENT ON TABLE platform.storage_backfill_checkpoints IS
				'Resumable per-tenant progress and verification evidence of storage backfills (#2580). Superuser CLI and migrations only.';

			-- Guardian values need reconciliation against their owners.
			-- Absence/status-day differences remain diagnostic: 1.15.398 copies
			-- and verifies the preserved flags and timestamps into Care Plan.
			ALTER TABLE platform.storage_backfill_checkpoints
				ADD COLUMN IF NOT EXISTS guardian_mismatch_count BIGINT NOT NULL DEFAULT 0,
				ADD COLUMN IF NOT EXISTS care_state_mismatch_count BIGINT NOT NULL DEFAULT 0,
				ADD COLUMN IF NOT EXISTS rows_visible_to_tenant BIGINT NOT NULL DEFAULT 0;
			COMMENT ON COLUMN platform.storage_backfill_checkpoints.guardian_mismatch_count IS
				'Rows whose legacy contact values have no counterpart in the owning tables. Only the student backfill (#2758) raises it.';
			COMMENT ON COLUMN platform.storage_backfill_checkpoints.care_state_mismatch_count IS
				'Rows whose legacy absence flags have no equivalent open status day. Only the student backfill (#2758) raises it.';
			COMMENT ON COLUMN platform.storage_backfill_checkpoints.rows_visible_to_tenant IS
				'Target rows the phoenix_tenant role saw for this school at the last verification; the copy is superuser and bypasses the policies. Only the student backfill (#2758) records it.';
		`)
		if err != nil {
			return fmt.Errorf("create storage backfill checkpoints: %w", err)
		}
		return provisionTenantRLS(ctx, tx, "platform.storage_backfill_checkpoints")
	})
	if err != nil {
		return err
	}
	report, err := RunStudentOwnerBackfill(ctx, db, StudentOwnerBackfillOptions{})
	if err != nil {
		return err
	}
	if !report.Stable() {
		// Concurrent old-table writes and unreconciled guardian values
		// keep a tenant unstable; the migration leaves its
		// checkpoint for the CLI instead of blocking deployment.
		migrationLog().WarnContext(ctx, "student owner backfill left unstable tenants; rerun `phoenix backfill student-owner` before Cutover",
			"unstable_tenants", report.Unstable(),
		)
	}
	return nil
}

// studentOwnerBackfillDown truncates only the target rows and removes this
// backfill's checkpoints so the Expand rollback can follow. The checkpoint
// table is shared with the other storage backfills and is dropped only when no
// other backfill has recorded progress in it; its two student-specific columns
// go with it rather than being dropped from a table others still use.
// users.students is never touched. It refuses after Cutover, when
// users.students is a compatibility view and the targets hold the authoritative
// data.
func studentOwnerBackfillDown(ctx context.Context, db *bun.DB) error {
	release, err := lockStorageBackfill(ctx, db, StudentOwnerBackfillName)
	if err != nil {
		return err
	}
	defer release()
	if err := assertStudentSourceIsBaseTable(ctx, db); err != nil {
		return err
	}
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `
			TRUNCATE users.student_care_profiles, users.student_school_memberships, users.student_profiles;
			DELETE FROM platform.storage_backfill_checkpoints WHERE backfill = ?;
			DO $$ BEGIN
				IF NOT EXISTS (SELECT 1 FROM platform.storage_backfill_checkpoints) THEN
					DROP TABLE platform.storage_backfill_checkpoints;
				END IF;
			END $$;
		`, StudentOwnerBackfillName)
		return err
	})
}
