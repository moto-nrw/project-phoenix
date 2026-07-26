package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	guardianChangeAuditVersion     = "1.15.144"
	guardianChangeAuditDescription = "Create audit trail for parent-portal changes to guardian contact data + pickup/emergency flags (#1667)."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     guardianChangeAuditVersion,
		Description: guardianChangeAuditDescription,
		DependsOn:   []string{grantGuardianEditPickupPermissionsVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.144: Creating audit.guardian_changes...")
			if _, err := db.NewRaw(`
				CREATE TABLE IF NOT EXISTS audit.guardian_changes (
					id BIGSERIAL PRIMARY KEY,
					tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
					student_id BIGINT NOT NULL REFERENCES users.students(id) ON DELETE CASCADE,
					guardian_profile_id BIGINT NOT NULL REFERENCES users.guardian_profiles(id) ON DELETE CASCADE,
					-- Nullable + ON DELETE SET NULL: the row snapshots the actor's
					-- name/email so the audit trail outlives a later account deletion
					-- rather than blocking it.
					actor_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
					actor_name_snapshot TEXT,
					actor_email_snapshot TEXT,
					-- 'contact' (profile field edit) or 'pickup' (per-child flag).
					change_type TEXT NOT NULL,
					field_name TEXT NOT NULL,
					-- Before/after as text: flags as 'true'/'false', a cleared
					-- contact field as NULL.
					old_value TEXT,
					new_value TEXT,
					changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
				);

				CREATE INDEX IF NOT EXISTS idx_guardian_changes_student
					ON audit.guardian_changes (tenant_id, student_id, changed_at DESC);
				CREATE INDEX IF NOT EXISTS idx_guardian_changes_guardian
					ON audit.guardian_changes (tenant_id, guardian_profile_id, changed_at DESC);
				CREATE INDEX IF NOT EXISTS idx_guardian_changes_actor
					ON audit.guardian_changes (actor_account_id, changed_at DESC);

				ALTER TABLE audit.guardian_changes ENABLE ROW LEVEL SECURITY;
				ALTER TABLE audit.guardian_changes FORCE ROW LEVEL SECURITY;
				DO $$
				BEGIN
					IF NOT EXISTS (
						SELECT 1
						FROM pg_policies
						WHERE schemaname = 'audit'
							AND tablename = 'guardian_changes'
							AND policyname = 'tenant_isolation_audit_guardian_changes'
					) THEN
						CREATE POLICY tenant_isolation_audit_guardian_changes
							ON audit.guardian_changes
							FOR ALL
							USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
							WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
					END IF;
				END $$;

				GRANT SELECT, INSERT ON audit.guardian_changes TO phoenix_tenant;
				GRANT USAGE ON SEQUENCE audit.guardian_changes_id_seq TO phoenix_tenant;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed creating audit.guardian_changes: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.144: Dropping audit.guardian_changes...")
			if _, err := db.NewRaw(`
				DROP TABLE IF EXISTS audit.guardian_changes CASCADE;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping audit.guardian_changes: %w", err)
			}
			return nil
		},
	)
}
