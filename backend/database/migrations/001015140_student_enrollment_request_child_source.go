package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	studentEnrollmentRequestChildSourceVersion     = "1.15.140"
	studentEnrollmentRequestChildSourceDescription = "Track enrollment request child provenance on materialized activity student enrollments."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentEnrollmentRequestChildSourceVersion,
		Description: studentEnrollmentRequestChildSourceDescription,
		DependsOn:   []string{enrollmentOfferingAdjustmentsVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return studentEnrollmentRequestChildSourceUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.140: Removing activities.student_enrollments enrollment_request_child_id...")
			if _, err := db.NewRaw(`
				DROP INDEX IF EXISTS activities.idx_student_enrollments_request_child;
				ALTER TABLE activities.student_enrollments
					DROP CONSTRAINT IF EXISTS fk_student_enrollments_enrollment_request_child,
					DROP COLUMN IF EXISTS enrollment_request_child_id;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed removing enrollment request child source from student enrollments: %w", err)
			}
			return nil
		},
	)
}

func studentEnrollmentRequestChildSourceUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.140: Adding activities.student_enrollments enrollment_request_child_id...")
	if _, err := db.NewRaw(`
		ALTER TABLE activities.student_enrollments
			ADD COLUMN IF NOT EXISTS enrollment_request_child_id BIGINT;

		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1
				FROM pg_constraint
				WHERE conname = 'fk_student_enrollments_enrollment_request_child'
					AND conrelid = 'activities.student_enrollments'::regclass
			) THEN
				ALTER TABLE activities.student_enrollments
					ADD CONSTRAINT fk_student_enrollments_enrollment_request_child
					FOREIGN KEY (enrollment_request_child_id)
					REFERENCES enrollment.request_children(id)
					ON DELETE SET NULL;
			END IF;
		END $$;

		CREATE INDEX IF NOT EXISTS idx_student_enrollments_request_child
			ON activities.student_enrollments (tenant_id, enrollment_request_child_id, student_id);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed adding enrollment request child source to student enrollments: %w", err)
	}
	return nil
}
