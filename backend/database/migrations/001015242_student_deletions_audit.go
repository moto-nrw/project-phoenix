package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	studentDeletionsAuditVersion     = "1.15.242"
	studentDeletionsAuditDescription = "Create data-minimal audit trail for permanent student deletion (#2068)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentDeletionsAuditVersion,
		Description: studentDeletionsAuditDescription,
		DependsOn:   []string{parentSuggestionsVersion},
	})

	Migrations.MustRegister(studentDeletionsAuditUp, studentDeletionsAuditDown)
}

func studentDeletionsAuditUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.242: Creating audit.student_deletions...")
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS audit.student_deletions (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			student_id BIGINT NOT NULL,
			actor_account_id BIGINT NOT NULL,
			reason TEXT NOT NULL CHECK (reason IN ('test_data', 'incorrect_entry', 'duplicate', 'privacy_request')),
			counts JSONB NOT NULL,
			deleted_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_student_deletions_student
			ON audit.student_deletions (tenant_id, student_id, deleted_at DESC);
		CREATE INDEX IF NOT EXISTS idx_student_deletions_actor
			ON audit.student_deletions (tenant_id, actor_account_id, deleted_at DESC);

		ALTER TABLE audit.student_deletions ENABLE ROW LEVEL SECURITY;
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_policies
				WHERE schemaname = 'audit'
				  AND tablename = 'student_deletions'
				  AND policyname = 'tenant_isolation_audit_student_deletions'
			) THEN
				CREATE POLICY tenant_isolation_audit_student_deletions ON audit.student_deletions
					FOR ALL
					USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
					WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
			END IF;
		END $$;
		ALTER TABLE audit.student_deletions FORCE ROW LEVEL SECURITY;

		GRANT SELECT, INSERT ON audit.student_deletions TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE audit.student_deletions_id_seq TO phoenix_tenant;
		REVOKE UPDATE, DELETE, TRUNCATE ON audit.student_deletions FROM phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("create audit.student_deletions: %w", err)
	}
	return nil
}

func studentDeletionsAuditDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.242: Dropping audit.student_deletions...")
	if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS audit.student_deletions CASCADE`); err != nil {
		return fmt.Errorf("drop audit.student_deletions: %w", err)
	}
	return nil
}
