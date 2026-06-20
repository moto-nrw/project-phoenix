package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	enrollmentOfferingAdjustmentsVersion     = "1.15.140"
	enrollmentOfferingAdjustmentsDescription = "Create audit trail for admin corrections to approved enrollment care offerings."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     enrollmentOfferingAdjustmentsVersion,
		Description: enrollmentOfferingAdjustmentsDescription,
		DependsOn:   []string{careOfferingAutoRulesVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.140: Creating audit.enrollment_offering_adjustments...")
			if _, err := db.NewRaw(`
				CREATE TABLE IF NOT EXISTS audit.enrollment_offering_adjustments (
					id BIGSERIAL PRIMARY KEY,
					tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
					request_id BIGINT NOT NULL REFERENCES enrollment.requests(id) ON DELETE CASCADE,
					request_child_id BIGINT NOT NULL REFERENCES enrollment.request_children(id) ON DELETE CASCADE,
					student_id BIGINT NOT NULL REFERENCES users.students(id) ON DELETE CASCADE,
					actor_account_id BIGINT NOT NULL REFERENCES auth.accounts(id),
					actor_role TEXT NOT NULL,
					actor_name_snapshot TEXT,
					actor_email_snapshot TEXT,
					reason TEXT NOT NULL,
					before_json JSONB NOT NULL DEFAULT '[]'::jsonb,
					after_json JSONB NOT NULL DEFAULT '[]'::jsonb,
					changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
				);

				CREATE INDEX IF NOT EXISTS idx_enrollment_offering_adjustments_child
					ON audit.enrollment_offering_adjustments (tenant_id, request_child_id, changed_at DESC);
				CREATE INDEX IF NOT EXISTS idx_enrollment_offering_adjustments_student
					ON audit.enrollment_offering_adjustments (tenant_id, student_id, changed_at DESC);
				CREATE INDEX IF NOT EXISTS idx_enrollment_offering_adjustments_actor
					ON audit.enrollment_offering_adjustments (actor_account_id, changed_at DESC);

				ALTER TABLE audit.enrollment_offering_adjustments ENABLE ROW LEVEL SECURITY;
				ALTER TABLE audit.enrollment_offering_adjustments FORCE ROW LEVEL SECURITY;
				DO $$
				BEGIN
					IF NOT EXISTS (
						SELECT 1
						FROM pg_policies
						WHERE schemaname = 'audit'
							AND tablename = 'enrollment_offering_adjustments'
							AND policyname = 'tenant_isolation_audit_enrollment_offering_adjustments'
					) THEN
						CREATE POLICY tenant_isolation_audit_enrollment_offering_adjustments
							ON audit.enrollment_offering_adjustments
							FOR ALL
							USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
							WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
					END IF;
				END $$;

				GRANT SELECT, INSERT ON audit.enrollment_offering_adjustments TO phoenix_tenant;
				GRANT USAGE ON SEQUENCE audit.enrollment_offering_adjustments_id_seq TO phoenix_tenant;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed creating audit.enrollment_offering_adjustments: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.140: Dropping audit.enrollment_offering_adjustments...")
			if _, err := db.NewRaw(`
				DROP TABLE IF EXISTS audit.enrollment_offering_adjustments CASCADE;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping audit.enrollment_offering_adjustments: %w", err)
			}
			return nil
		},
	)
}
