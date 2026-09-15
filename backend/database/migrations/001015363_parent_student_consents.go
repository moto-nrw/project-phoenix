package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	parentStudentConsentsVersion     = "1.15.363"
	parentStudentConsentsDescription = "Add append-only student consent history and parent consent permission (#2636)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentStudentConsentsVersion,
		Description: parentStudentConsentsDescription,
		DependsOn:   []string{leasedDeliveryOutboxVersion},
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

		GRANT SELECT, INSERT ON audit.student_consent_changes TO phoenix_tenant;
		REVOKE UPDATE, DELETE, TRUNCATE ON audit.student_consent_changes FROM phoenix_tenant;
		GRANT USAGE ON SEQUENCE audit.student_consent_changes_id_seq TO phoenix_tenant;

		CREATE TABLE meta.parent_student_consent_permission_grants (
			student_guardian_id BIGINT PRIMARY KEY
				REFERENCES users.students_guardians(id) ON DELETE CASCADE
		);

		INSERT INTO meta.parent_student_consent_permission_grants (student_guardian_id)
		SELECT id
		FROM users.students_guardians
		WHERE guardian_role IN ('primary_guardian', 'legal_guardian', 'co_guardian')
			AND NOT (COALESCE(permissions, '{}'::jsonb) ? 'parent_portal.consent.manage');

		UPDATE users.students_guardians AS student_guardian
		SET permissions = COALESCE(permissions, '{}'::jsonb)
			|| '{"parent_portal.consent.manage": true}'::jsonb
		FROM meta.parent_student_consent_permission_grants AS migration_grant
		WHERE student_guardian.id = migration_grant.student_guardian_id;

		CREATE FUNCTION meta.invalidate_parent_student_consent_permission_grant()
		RETURNS TRIGGER
		LANGUAGE plpgsql
		SECURITY DEFINER
		SET search_path = pg_catalog, meta
		AS $$
		BEGIN
			DELETE FROM meta.parent_student_consent_permission_grants
			WHERE student_guardian_id = OLD.id;
			RETURN NEW;
		END;
		$$;
		CREATE TRIGGER invalidate_parent_student_consent_permission_grant
			AFTER UPDATE OF permissions ON users.students_guardians
			FOR EACH ROW
			WHEN (
				(OLD.permissions -> 'parent_portal.consent.manage')
				IS DISTINCT FROM
				(NEW.permissions -> 'parent_portal.consent.manage')
			)
			EXECUTE FUNCTION meta.invalidate_parent_student_consent_permission_grant();
		REVOKE ALL ON FUNCTION meta.invalidate_parent_student_consent_permission_grant() FROM PUBLIC;

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
	return provisionTenantRLS(ctx, db, "audit.student_consent_changes")
}

func parentStudentConsentsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.NewRaw(`
		DROP TRIGGER IF EXISTS invalidate_parent_student_consent_permission_grant
			ON users.students_guardians;
		DROP FUNCTION IF EXISTS meta.invalidate_parent_student_consent_permission_grant();

		UPDATE users.students_guardians AS student_guardian
		SET permissions = student_guardian.permissions - 'parent_portal.consent.manage'
		FROM meta.parent_student_consent_permission_grants AS migration_grant
		WHERE student_guardian.id = migration_grant.student_guardian_id
			AND student_guardian.permissions ? 'parent_portal.consent.manage';

		DROP TABLE meta.parent_student_consent_permission_grants;
		DROP TABLE IF EXISTS audit.student_consent_changes CASCADE;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("drop parent student consent history: %w", err)
	}
	return nil
}
