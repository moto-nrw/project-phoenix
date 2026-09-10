package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/uptrace/bun"
)

const staffOwnerBackfillVersion = "1.15.381"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffOwnerBackfillVersion,
		Description: "Backfill Membership and Workforce staff storage from users.staff in resumable tenant batches (#2752)",
		DependsOn:   []string{"1.15.376"},
	})
	Migrations.MustRegister(staffOwnerBackfillUp, staffOwnerBackfillDown)
}

// staffOwnerBackfillUp creates the checkpoint storage and runs the initial
// copy. Every batch commits on its own, so an interrupted deployment simply
// resumes at the persisted high-water mark on the next run; that is why the
// DDL is idempotent. users.staff stays authoritative: no caller switches, no
// trigger or dual write is introduced, and the old rows are never modified.
// Remaining source writes are picked up by `phoenix backfill staff-owner`
// before Cutover (#2753), which applies the final delta under its own lock.
func staffOwnerBackfillUp(ctx context.Context, db *bun.DB) error {
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
				verified_at TIMESTAMPTZ,
				stable_at TIMESTAMPTZ,
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				PRIMARY KEY (backfill, tenant_id)
			);
			COMMENT ON TABLE platform.storage_backfill_checkpoints IS
				'Resumable per-tenant progress and verification evidence of storage backfills (#2580). Superuser CLI and migrations only.';
		`)
		if err != nil {
			return fmt.Errorf("create storage backfill checkpoints: %w", err)
		}
		return provisionTenantRLS(ctx, tx, "platform.storage_backfill_checkpoints")
	})
	if err != nil {
		return err
	}
	report, err := RunStaffOwnerBackfill(ctx, db, StaffOwnerBackfillOptions{})
	if err != nil {
		return err
	}
	if !report.Stable() {
		// Concurrent old-table writes keep a tenant unstable; the migration
		// leaves its checkpoint for the CLI instead of blocking deployment.
		slog.Warn("staff owner backfill left unstable tenants; rerun `phoenix backfill staff-owner` before Cutover",
			"migration", staffOwnerBackfillVersion,
			"unstable_tenants", report.Unstable())
	}
	return nil
}

// staffOwnerBackfillDown truncates only the target rows and removes this
// backfill's checkpoints so the Expand rollback can follow. The checkpoint
// table is shared with later backfills and is dropped only when no other
// backfill has recorded progress in it. users.staff is never touched. It
// refuses after Cutover, when users.staff is a compatibility view and the
// targets hold the authoritative data.
func staffOwnerBackfillDown(ctx context.Context, db *bun.DB) error {
	if err := assertStaffSourceIsBaseTable(ctx, db); err != nil {
		return err
	}
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `
			TRUNCATE users.staff_employment_profiles, users.staff_school_memberships;
			DELETE FROM platform.storage_backfill_checkpoints WHERE backfill = ?;
			DO $$ BEGIN
				IF NOT EXISTS (SELECT 1 FROM platform.storage_backfill_checkpoints) THEN
					DROP TABLE platform.storage_backfill_checkpoints;
				END IF;
			END $$;
		`, StaffOwnerBackfillName)
		return err
	})
}
