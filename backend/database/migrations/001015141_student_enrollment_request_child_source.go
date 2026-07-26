package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	studentEnrollmentRequestChildSourceVersion     = "1.15.141"
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
			fmt.Println("Rolling back migration 1.15.141: Removing activities.student_enrollments enrollment_request_child_id...")
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
	fmt.Println("Migration 1.15.141: Adding activities.student_enrollments enrollment_request_child_id...")
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
	if err := backfillStudentEnrollmentRequestChildSource(ctx, db); err != nil {
		return err
	}
	return nil
}

func backfillStudentEnrollmentRequestChildSource(ctx context.Context, db *bun.DB) error {
	if _, err := db.NewRaw(`
		WITH candidates AS (
			SELECT
				"student_enrollment".id AS enrollment_id,
				"request_child".id AS request_child_id
			FROM activities.student_enrollments AS "student_enrollment"
			INNER JOIN enrollment.request_children AS "request_child"
				ON "request_child".tenant_id = "student_enrollment".tenant_id
				AND "request_child".created_student_id = "student_enrollment".student_id
				AND "request_child".status = 'approved'
				AND "request_child".reviewed_at IS NOT NULL
				AND "student_enrollment".created_at BETWEEN "request_child".reviewed_at - INTERVAL '5 minutes'
					AND "request_child".reviewed_at + INTERVAL '5 minutes'
			INNER JOIN enrollment.requests AS "request"
				ON "request".tenant_id = "student_enrollment".tenant_id
				AND "request".id = "request_child".request_id
			INNER JOIN enrollment.phases AS "phase"
				ON "phase".tenant_id = "student_enrollment".tenant_id
				AND "phase".id = "request".phase_id
				AND "student_enrollment".valid_from = "phase".service_start_date
				AND "student_enrollment".valid_until IS NOT DISTINCT FROM "phase".service_end_date
			INNER JOIN enrollment.request_child_offerings AS "request_child_offering"
				ON "request_child_offering".tenant_id = "student_enrollment".tenant_id
				AND "request_child_offering".request_child_id = "request_child".id
			INNER JOIN enrollment.care_offerings AS "care_offering"
				ON "care_offering".tenant_id = "student_enrollment".tenant_id
				AND "care_offering".id = "request_child_offering".care_offering_id
				AND "care_offering".activity_group_id = "student_enrollment".activity_group_id
			WHERE "student_enrollment".enrollment_request_child_id IS NULL
		),
		unambiguous AS (
			SELECT enrollment_id, MIN(request_child_id) AS request_child_id
			FROM candidates
			GROUP BY enrollment_id
			HAVING COUNT(DISTINCT request_child_id) = 1
		)
		UPDATE activities.student_enrollments AS "student_enrollment"
		SET enrollment_request_child_id = "unambiguous".request_child_id
		FROM unambiguous AS "unambiguous"
		WHERE "student_enrollment".id = "unambiguous".enrollment_id
			AND "student_enrollment".enrollment_request_child_id IS NULL;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed backfilling enrollment request child source on student enrollments: %w", err)
	}
	return nil
}
