package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	timeTrackingDeletionsAuditVersion     = "1.15.227"
	timeTrackingDeletionsAuditDescription = "Create audit.time_tracking_deletions: append-only tombstones for deleted balance adjustments and staff absences (#1417)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     timeTrackingDeletionsAuditVersion,
		Description: timeTrackingDeletionsAuditDescription,
		DependsOn: []string{
			auditAppendOnlyGrantsVersion,
			staffBalanceAdjustmentsVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return timeTrackingDeletionsAuditUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return timeTrackingDeletionsAuditDown(ctx, db)
		},
	)
}

// timeTrackingDeletionsAuditUp closes the audit hole found while building the
// cross-staff audit log (#1417): DELETE on active.staff_balance_adjustments
// removed the ledger row without a trace, and the admin delete of
// active.staff_absences cascaded away the absence's own audit trail
// (active.staff_absence_audit is ON DELETE CASCADE). Both deletes now write a
// tombstone here, in the same tenant transaction, before the row disappears.
//
// The payload column stores the full deleted row as JSONB. Unlike
// audit.enrollment_deletions (data-minimal by design, guardians' data), these
// rows are exactly the working-time records §16 Abs. 2 ArbZG requires an
// employer to retain — dropping the values would defeat the tombstone.
//
// staff_id and deleted_by carry no FK on purpose, matching
// audit.work_session_edits: the trail must survive offboarding of either
// person. Append-only for phoenix_tenant, like the rest of the audit schema
// (1.15.225).
func timeTrackingDeletionsAuditUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.227: Creating audit.time_tracking_deletions...")

	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS audit.time_tracking_deletions (
			id          BIGSERIAL PRIMARY KEY,
			tenant_id   BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			staff_id    BIGINT NOT NULL,
			source      TEXT NOT NULL CHECK (source IN ('balance_adjustment', 'absence')),
			source_id   BIGINT NOT NULL,
			deleted_by  BIGINT NOT NULL,
			payload     JSONB NOT NULL,
			note        TEXT NOT NULL DEFAULT '',
			occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_time_tracking_deletions_tenant_occurred
			ON audit.time_tracking_deletions (tenant_id, occurred_at DESC);

		ALTER TABLE audit.time_tracking_deletions ENABLE ROW LEVEL SECURITY;
		CREATE POLICY tenant_isolation_time_tracking_deletions ON audit.time_tracking_deletions
			USING (tenant_id = current_setting('app.current_tenant_id')::BIGINT);
		ALTER TABLE audit.time_tracking_deletions FORCE ROW LEVEL SECURITY;

		GRANT SELECT, INSERT ON audit.time_tracking_deletions TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE audit.time_tracking_deletions_id_seq TO phoenix_tenant;
		REVOKE UPDATE, DELETE ON audit.time_tracking_deletions FROM phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error creating audit.time_tracking_deletions: %w", err)
	}
	return nil
}

func timeTrackingDeletionsAuditDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.227: Dropping audit.time_tracking_deletions...")
	_, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS audit.time_tracking_deletions;`)
	if err != nil {
		return fmt.Errorf("error dropping audit.time_tracking_deletions: %w", err)
	}
	return nil
}
