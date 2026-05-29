package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	createEnrollmentRequestChildrenVersion     = "1.15.61"
	createEnrollmentRequestChildrenDescription = "Create enrollment.request_children table - per-child rows for each enrollment request (PR 5)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     createEnrollmentRequestChildrenVersion,
		Description: createEnrollmentRequestChildrenDescription,
		DependsOn: []string{
			createEnrollmentRequestsVersion,
			UsersStudentsVersion, // created_student_id FK target
			"1.0.1",              // auth.accounts (reviewed_by FK)
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createEnrollmentRequestChildrenUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return createEnrollmentRequestChildrenDown(ctx, db)
		},
	)
}

func createEnrollmentRequestChildrenUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.61: Creating enrollment.request_children table...")

	if _, err := db.NewRaw(`
		CREATE TABLE IF NOT EXISTS enrollment.request_children (
			id                  BIGSERIAL PRIMARY KEY,
			tenant_id           BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			request_id          BIGINT NOT NULL REFERENCES enrollment.requests(id) ON DELETE CASCADE,
			first_name          TEXT NOT NULL,
			last_name           TEXT NOT NULL,
			date_of_birth       DATE NOT NULL,
			target_grade_level  SMALLINT
				CHECK (target_grade_level IS NULL OR (target_grade_level BETWEEN 1 AND 13)),
			custom_data         JSONB NOT NULL DEFAULT '{}'::jsonb,
			status              TEXT NOT NULL DEFAULT 'submitted'
				CHECK (status IN ('submitted', 'under_review', 'approved', 'waitlisted', 'rejected', 'withdrawn')),
			status_reason       TEXT,
			activation_mode     TEXT NOT NULL DEFAULT 'scheduled'
				CHECK (activation_mode IN ('immediate', 'scheduled')),
			activate_on         DATE,
			reviewed_at         TIMESTAMPTZ,
			reviewed_by         BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
			created_student_id  BIGINT REFERENCES users.students(id) ON DELETE SET NULL,
			sort_order          INT NOT NULL DEFAULT 0
		);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating enrollment.request_children: %w", err)
	}

	if _, err := db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_enrollment_request_children_request
		ON enrollment.request_children (request_id, sort_order);

		CREATE INDEX IF NOT EXISTS idx_enrollment_request_children_tenant_status
		ON enrollment.request_children (tenant_id, status);

		CREATE INDEX IF NOT EXISTS idx_enrollment_request_children_created_student
		ON enrollment.request_children (created_student_id)
		WHERE created_student_id IS NOT NULL;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating enrollment.request_children indexes: %w", err)
	}

	if _, err := db.NewRaw(`
		ALTER TABLE enrollment.request_children ENABLE ROW LEVEL SECURITY;
		ALTER TABLE enrollment.request_children FORCE ROW LEVEL SECURITY;
		CREATE POLICY tenant_isolation_enrollment_request_children ON enrollment.request_children
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed enabling RLS on enrollment.request_children: %w", err)
	}

	return nil
}

func createEnrollmentRequestChildrenDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.61: Dropping enrollment.request_children...")
	if _, err := db.NewRaw(`DROP TABLE IF EXISTS enrollment.request_children CASCADE;`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping enrollment.request_children: %w", err)
	}
	return nil
}
