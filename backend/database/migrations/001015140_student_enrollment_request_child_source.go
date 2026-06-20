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

				WITH candidates AS (
					SELECT
						se.id AS student_enrollment_id,
						rc.id AS request_child_id,
						ROW_NUMBER() OVER (
							PARTITION BY se.id
							ORDER BY rc.id DESC
						) AS rn
					FROM activities.student_enrollments AS se
					INNER JOIN enrollment.request_children AS rc
						ON rc.tenant_id = se.tenant_id
						AND rc.created_student_id = se.student_id
					INNER JOIN enrollment.requests AS req
						ON req.tenant_id = rc.tenant_id
						AND req.id = rc.request_id
					INNER JOIN enrollment.phases AS phase
						ON phase.tenant_id = req.tenant_id
						AND phase.id = req.phase_id
					INNER JOIN enrollment.request_child_offerings AS rco
						ON rco.tenant_id = rc.tenant_id
						AND rco.request_child_id = rc.id
					INNER JOIN enrollment.care_offerings AS offering
						ON offering.tenant_id = rco.tenant_id
						AND offering.id = rco.care_offering_id
					WHERE se.enrollment_request_child_id IS NULL
						AND rc.created_student_id IS NOT NULL
						AND offering.activity_group_id = se.activity_group_id
						AND se.valid_from = phase.service_start_date
						AND se.valid_until IS NOT DISTINCT FROM phase.service_end_date
				)
				UPDATE activities.student_enrollments AS se
				SET enrollment_request_child_id = candidates.request_child_id
				FROM candidates
				WHERE candidates.rn = 1
					AND se.id = candidates.student_enrollment_id
					AND se.enrollment_request_child_id IS NULL;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding enrollment request child source to student enrollments: %w", err)
			}
			return nil
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
