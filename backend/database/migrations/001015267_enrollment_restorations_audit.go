package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	enrollmentRestorationsAuditVersion     = "1.15.267"
	enrollmentRestorationsAuditDescription = "Create audit.enrollment_restorations: append-only trail for admin-restored withdrawn enrollment requests (#2157)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     enrollmentRestorationsAuditVersion,
		Description: enrollmentRestorationsAuditDescription,
		DependsOn: []string{
			groupTargetsVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return enrollmentRestorationsAuditUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return enrollmentRestorationsAuditDown(ctx, db)
		},
	)
}

// enrollmentRestorationsAuditUp backs the admin "Anmeldung wiederherstellen"
// action (#2157): undoing a parent's accidental withdraw previously required
// manual SQL on the production DB, which left no trace of who restored what.
// Every restore now writes one row here, in the same tenant transaction as the
// status flip.
//
// Data-minimal like audit.enrollment_deletions (1.15.200): identifiers only,
// no names or contact data. child_ids carries the restored request_children
// ids as a JSONB array. actor_account_id is SET NULL on account deletion so
// the trail survives offboarding. Append-only for phoenix_tenant, like the
// rest of the audit schema (1.15.225).
func enrollmentRestorationsAuditUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.267: Creating audit.enrollment_restorations...")

	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS audit.enrollment_restorations (
			id               BIGSERIAL PRIMARY KEY,
			tenant_id        BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			request_id       BIGINT NOT NULL,
			child_ids        JSONB NOT NULL,
			actor_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
			restored_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_enrollment_restorations_request
			ON audit.enrollment_restorations (tenant_id, request_id, restored_at DESC);

		ALTER TABLE audit.enrollment_restorations ENABLE ROW LEVEL SECURITY;
		CREATE POLICY tenant_isolation_enrollment_restorations ON audit.enrollment_restorations
			USING (tenant_id = current_setting('app.current_tenant_id')::BIGINT);
		ALTER TABLE audit.enrollment_restorations FORCE ROW LEVEL SECURITY;

		GRANT SELECT, INSERT ON audit.enrollment_restorations TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE audit.enrollment_restorations_id_seq TO phoenix_tenant;
		REVOKE UPDATE, DELETE ON audit.enrollment_restorations FROM phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error creating audit.enrollment_restorations: %w", err)
	}
	return nil
}

func enrollmentRestorationsAuditDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.267: Dropping audit.enrollment_restorations...")
	_, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS audit.enrollment_restorations;`)
	if err != nil {
		return fmt.Errorf("error dropping audit.enrollment_restorations: %w", err)
	}
	return nil
}
