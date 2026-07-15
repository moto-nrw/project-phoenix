package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	enrollmentDeletionsAuditVersion     = "1.15.200"
	enrollmentDeletionsAuditDescription = "Create append-only audit trail for controlled enrollment deletion (#1921)."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     enrollmentDeletionsAuditVersion,
		Description: enrollmentDeletionsAuditDescription,
		DependsOn:   []string{sickAbsenceProvenanceVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.200: Creating audit.enrollment_deletions...")
			if _, err := db.NewRaw(`
				CREATE TABLE IF NOT EXISTS audit.enrollment_deletions (
					id BIGSERIAL PRIMARY KEY,
					tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
					request_id BIGINT NOT NULL,
					child_id BIGINT,
					actor_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
					actor_type TEXT NOT NULL CHECK (actor_type IN ('admin', 'system')),
					scope TEXT NOT NULL CHECK (scope IN ('request', 'child')),
					reason TEXT NOT NULL CHECK (char_length(btrim(reason)) BETWEEN 3 AND 500),
					counts JSONB NOT NULL,
					deleted_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
				);

				CREATE INDEX IF NOT EXISTS idx_enrollment_deletions_request
					ON audit.enrollment_deletions (tenant_id, request_id, deleted_at DESC);
				CREATE INDEX IF NOT EXISTS idx_enrollment_deletions_actor
					ON audit.enrollment_deletions (actor_account_id, deleted_at DESC)
					WHERE actor_account_id IS NOT NULL;

				ALTER TABLE audit.enrollment_deletions ENABLE ROW LEVEL SECURITY;
				ALTER TABLE audit.enrollment_deletions FORCE ROW LEVEL SECURITY;
				DO $$
				BEGIN
					IF NOT EXISTS (
						SELECT 1 FROM pg_policies
						WHERE schemaname = 'audit'
						  AND tablename = 'enrollment_deletions'
						  AND policyname = 'tenant_isolation_audit_enrollment_deletions'
					) THEN
						CREATE POLICY tenant_isolation_audit_enrollment_deletions
							ON audit.enrollment_deletions
							FOR ALL
							USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
							WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
					END IF;
				END $$;

				GRANT SELECT, INSERT ON audit.enrollment_deletions TO phoenix_tenant;
				-- The schema-wide default ACL from 1.14.1 auto-grants phoenix_tenant
				-- UPDATE/DELETE on new audit tables; revoke to keep this append-only.
				REVOKE UPDATE, DELETE ON audit.enrollment_deletions FROM phoenix_tenant;
				GRANT USAGE ON SEQUENCE audit.enrollment_deletions_id_seq TO phoenix_tenant;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("create audit.enrollment_deletions: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.200: Dropping audit.enrollment_deletions...")
			if _, err := db.NewRaw(`DROP TABLE IF EXISTS audit.enrollment_deletions CASCADE`).Exec(ctx); err != nil {
				return fmt.Errorf("drop audit.enrollment_deletions: %w", err)
			}
			return nil
		},
	)
}
