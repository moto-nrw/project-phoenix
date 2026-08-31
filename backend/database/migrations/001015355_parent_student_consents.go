package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	parentStudentConsentsVersion     = "1.15.355"
	parentStudentConsentsDescription = "Add append-only student consent history and parent consent permission (#2636)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentStudentConsentsVersion,
		Description: parentStudentConsentsDescription,
		DependsOn:   []string{additionalSupervisionAuditVersion},
	})
	Migrations.MustRegister(parentStudentConsentsUp, parentStudentConsentsDown)
}

func parentStudentConsentsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.NewRaw(`
		CREATE TABLE audit.student_consent_changes (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			student_id BIGINT NOT NULL REFERENCES users.students(id) ON DELETE CASCADE,
			consent_key TEXT NOT NULL CHECK (consent_key IN ('agb', 'data_processing', 'email_contact', 'photo')),
			action TEXT NOT NULL CHECK (action IN ('granted', 'withdrawn')),
			source TEXT NOT NULL CHECK (source IN ('enrollment', 'tenant_portal', 'parent_portal', 'import', 'migration_snapshot')),
			actor_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE INDEX idx_student_consent_changes_student
			ON audit.student_consent_changes (tenant_id, student_id, created_at DESC, id DESC);
		CREATE INDEX idx_student_consent_changes_student_fk
			ON audit.student_consent_changes (student_id);
		CREATE INDEX idx_student_consent_changes_actor
			ON audit.student_consent_changes (actor_account_id, created_at DESC)
			WHERE actor_account_id IS NOT NULL;

		ALTER TABLE audit.student_consent_changes ENABLE ROW LEVEL SECURITY;
		ALTER TABLE audit.student_consent_changes FORCE ROW LEVEL SECURITY;
		CREATE POLICY tenant_isolation_audit_student_consent_changes
			ON audit.student_consent_changes
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT ON audit.student_consent_changes TO phoenix_tenant;
		REVOKE UPDATE, DELETE, TRUNCATE ON audit.student_consent_changes FROM phoenix_tenant;
		GRANT USAGE ON SEQUENCE audit.student_consent_changes_id_seq TO phoenix_tenant;

		UPDATE users.students_guardians
		SET permissions = COALESCE(permissions, '{}'::jsonb)
			|| '{"parent_portal.consent.manage": true}'::jsonb
		WHERE guardian_role IN ('primary_guardian', 'legal_guardian', 'co_guardian');

		INSERT INTO audit.student_consent_changes
			(tenant_id, student_id, consent_key, action, source, actor_account_id, created_at, updated_at)
		SELECT tenant_id, id, 'agb', 'granted', 'migration_snapshot', NULL::BIGINT, agb_accepted_at, agb_accepted_at
		FROM users.students
		WHERE agb_accepted_at IS NOT NULL
		UNION ALL
		SELECT tenant_id, id, 'data_processing', 'granted', 'migration_snapshot', NULL::BIGINT, data_processing_accepted_at, data_processing_accepted_at
		FROM users.students
		WHERE data_processing_accepted_at IS NOT NULL
		UNION ALL
		SELECT tenant_id, id, 'email_contact', 'granted', 'migration_snapshot', NULL::BIGINT, email_contact_accepted_at, email_contact_accepted_at
		FROM users.students
		WHERE email_contact_accepted_at IS NOT NULL
		UNION ALL
		SELECT tenant_id, id, 'photo', 'granted', 'migration_snapshot', photo_consent_given_by, photo_consent_given_at, photo_consent_given_at
		FROM users.students
		WHERE photo_consent_given_at IS NOT NULL;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("create parent student consent history: %w", err)
	}
	return nil
}

func parentStudentConsentsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.NewRaw(`
		UPDATE users.students_guardians
		SET permissions = permissions - 'parent_portal.consent.manage'
		WHERE permissions ? 'parent_portal.consent.manage';

		DROP TABLE IF EXISTS audit.student_consent_changes CASCADE;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("drop parent student consent history: %w", err)
	}
	return nil
}
