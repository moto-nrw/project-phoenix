package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	submissionRateLimitsBackfillRLSVersion     = "1.15.77"
	submissionRateLimitsBackfillRLSDescription = "Backfill RLS policy on enrollment.submission_rate_limits for databases that ran the pre-amendment 1.15.66 (which created the table without RLS). Idempotent: no-op on databases that already have the policy from the amended 1.15.66."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     submissionRateLimitsBackfillRLSVersion,
		Description: submissionRateLimitsBackfillRLSDescription,
		DependsOn: []string{
			submissionRateLimitsAddUpdatedAtVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.77: Backfilling RLS policy on enrollment.submission_rate_limits...")

			// ENABLE / FORCE ROW LEVEL SECURITY are idempotent: re-running
			// them on a table that already has RLS active is a no-op.
			// DROP POLICY IF EXISTS + CREATE POLICY guarantees the policy
			// matches the canonical definition even if a partial / stale
			// version is sitting on the table.
			//
			// Background: migration 1.15.66 originally created
			// enrollment.submission_rate_limits without RLS. The
			// amendment to 1.15.66 added RLS in-place, but the bun
			// migration registry skips migrations whose version is
			// already in bun_migrations, so any DB that ran the
			// pre-amendment 1.15.66 never got the policy. This
			// migration closes that drift between fresh and pre-existing
			// databases.
			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.submission_rate_limits ENABLE ROW LEVEL SECURITY;
				ALTER TABLE enrollment.submission_rate_limits FORCE ROW LEVEL SECURITY;
				DROP POLICY IF EXISTS tenant_isolation_submission_rate_limits ON enrollment.submission_rate_limits;
				CREATE POLICY tenant_isolation_submission_rate_limits ON enrollment.submission_rate_limits
					FOR ALL
					USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
					WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed backfilling RLS on enrollment.submission_rate_limits: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.77: dropping tenant_isolation_submission_rate_limits policy...")
			// Best-effort rollback: drop the policy but leave RLS enabled
			// (other migrations may rely on the FORCE flag).
			if _, err := db.NewRaw(`
				DROP POLICY IF EXISTS tenant_isolation_submission_rate_limits ON enrollment.submission_rate_limits;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping tenant_isolation_submission_rate_limits policy: %w", err)
			}
			return nil
		},
	)
}
