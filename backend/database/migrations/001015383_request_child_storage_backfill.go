package migrations

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/uptrace/bun"
)

const requestChildStorageBackfillVersion = "1.15.383"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     requestChildStorageBackfillVersion,
		Description: "Backfill and verify submitted offering selections and effective care bookings in resumable tenant batches (#2713)",
		DependsOn:   []string{requestChildStorageExpandVersion},
	})
	Migrations.MustRegister(requestChildStorageBackfillUp, requestChildStorageBackfillDown)
}

// requestChildStorageBackfillUp runs outside a single transaction on purpose:
// the checkpoint DDL is idempotent and every copy batch commits on its own, so
// an interrupted deployment resumes from the persisted high-water mark on the
// next run instead of repeating finished work. The legacy table is read only.
func requestChildStorageBackfillUp(ctx context.Context, db *bun.DB) error {
	if err := createRequestChildStorageCheckpoints(ctx, db); err != nil {
		return err
	}
	slog.Info("migration starting",
		"migration", requestChildStorageBackfillVersion)
	report, err := RunRequestChildStorageBackfill(ctx, db, RequestChildStorageBackfillOptions{})
	if err != nil {
		return fmt.Errorf("backfill request child storage: %w", err)
	}
	slog.Info("migration finished",
		"migration", requestChildStorageBackfillVersion,
		"tenants", len(report.Tenants),
		"database_deadlocks", report.DatabaseDeadlocks,
		"duration_ms", report.Duration.Milliseconds())
	// Deployments migrate with the application stopped, so a residual
	// mismatch is a copy defect, not concurrent traffic. Fail loudly instead
	// of leaving Cutover an incomplete checkpoint.
	if incomplete := report.IncompleteTenants(); len(incomplete) > 0 {
		return fmt.Errorf("backfill request child storage: tenants %v did not reach equal counts and checksums", incomplete)
	}
	return nil
}

func createRequestChildStorageCheckpoints(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS enrollment.request_child_storage_backfill_checkpoints (
				tenant_id BIGINT PRIMARY KEY REFERENCES platform.schools(id) ON DELETE CASCADE,
				high_water_mark BIGINT NOT NULL DEFAULT 0,
				rows_scanned BIGINT NOT NULL DEFAULT 0,
				rows_copied BIGINT NOT NULL DEFAULT 0,
				rows_skipped BIGINT NOT NULL DEFAULT 0,
				rows_deleted BIGINT NOT NULL DEFAULT 0,
				batches_completed BIGINT NOT NULL DEFAULT 0,
				batches_retried BIGINT NOT NULL DEFAULT 0,
				deadlocks BIGINT NOT NULL DEFAULT 0,
				serialization_failures BIGINT NOT NULL DEFAULT 0,
				lock_timeouts BIGINT NOT NULL DEFAULT 0,
				lock_wait_ms DOUBLE PRECISION NOT NULL DEFAULT 0,
				verification_snapshot TEXT NOT NULL DEFAULT '',
				provenance_policy TEXT NOT NULL DEFAULT '',
				unresolved_origins BIGINT NOT NULL DEFAULT 0,
				unresolved_reasons JSONB NOT NULL DEFAULT '{}'::jsonb,
				batch_p95_ms BIGINT NOT NULL DEFAULT 0,
				batch_max_ms BIGINT NOT NULL DEFAULT 0,
				pool_wait_ms BIGINT NOT NULL DEFAULT 0,
				passes INTEGER NOT NULL DEFAULT 0,
				stable BOOLEAN NOT NULL DEFAULT FALSE,
				source_bookings BIGINT,
				target_bookings BIGINT,
				source_selections BIGINT,
				target_selections BIGINT,
				source_bookings_checksum TEXT,
				target_bookings_checksum TEXT,
				source_selections_checksum TEXT,
				target_selections_checksum TEXT,
				mismatch_count BIGINT,
				orphan_count BIGINT,
				oldest_unmigrated_seconds BIGINT,
				complete BOOLEAN NOT NULL DEFAULT FALSE,
				runs INTEGER NOT NULL DEFAULT 0,
				last_run_started_at TIMESTAMPTZ,
				verify_only_at TIMESTAMPTZ,
				verify_only_mismatch_count BIGINT,
				started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				verified_at TIMESTAMPTZ
			);
			REVOKE ALL ON enrollment.request_child_storage_backfill_checkpoints FROM phoenix_tenant;
			GRANT SELECT ON enrollment.request_child_storage_backfill_checkpoints TO phoenix_admin;
		`)
		if err != nil {
			return fmt.Errorf("create request child storage backfill checkpoints: %w", err)
		}
		// The job runs as superuser; tenant roles get no grant. RLS still labels
		// the table as tenant-scoped like every other tenant_id table.
		return provisionTenantRLS(ctx, tx, "enrollment.request_child_storage_backfill_checkpoints")
	})
}

// requestChildStorageBackfillDown is the pre-Cutover rollback: it removes only
// copied target rows and the checkpoints. Legacy rows are never touched.
func requestChildStorageBackfillDown(ctx context.Context, db *bun.DB) error {
	release, err := lockRequestChildStorageBackfill(ctx, db)
	if err != nil {
		return err
	}
	defer release()
	if err := assertRequestChildStorageSource(ctx, db); err != nil {
		return err
	}
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `
			TRUNCATE enrollment.care_offering_bookings, enrollment.request_child_offering_selections;
			DROP TABLE IF EXISTS enrollment.request_child_storage_backfill_checkpoints;
		`)
		if err != nil {
			return fmt.Errorf("roll back request child storage backfill: %w", err)
		}
		return nil
	})
}
