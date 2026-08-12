package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	partialStudentAbsencesVersion     = "1.15.283"
	partialStudentAbsencesDescription = "Use pickup exceptions as the source for one-day excused absences from a wall-clock time (#2216)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     partialStudentAbsencesVersion,
		Description: partialStudentAbsencesDescription,
		DependsOn:   []string{attendancePerCareSlotVersion},
	})
	Migrations.MustRegister(partialStudentAbsencesUp, partialStudentAbsencesDown)
}

func partialStudentAbsencesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.283: Adding partial student absence fields (#2216)...")

	_, err := db.NewRaw(`
		ALTER TABLE schedule.student_pickup_exceptions
			ADD COLUMN IF NOT EXISTS excused_from TIME WITHOUT TIME ZONE,
			ADD COLUMN IF NOT EXISTS excused_reason TEXT,
			ADD COLUMN IF NOT EXISTS excused_created_by BIGINT,
			ADD COLUMN IF NOT EXISTS excused_owns_pickup_time BOOLEAN NOT NULL DEFAULT FALSE;

		ALTER TABLE schedule.student_pickup_exceptions
			DROP CONSTRAINT IF EXISTS chk_pickup_exception_partial_absence;
		ALTER TABLE schedule.student_pickup_exceptions
			ADD CONSTRAINT chk_pickup_exception_partial_absence CHECK (
				(excused_from IS NULL AND excused_reason IS NULL AND excused_created_by IS NULL AND NOT excused_owns_pickup_time)
				OR (excused_from IS NOT NULL AND excused_created_by IS NOT NULL AND (NOT excused_owns_pickup_time OR pickup_time IS NOT NULL))
			);
		ALTER TABLE schedule.student_pickup_exceptions
			DROP CONSTRAINT IF EXISTS fk_pickup_exception_excused_created_by_tenant;
		ALTER TABLE schedule.student_pickup_exceptions
			ADD CONSTRAINT fk_pickup_exception_excused_created_by_tenant
			FOREIGN KEY (tenant_id, excused_created_by)
			REFERENCES users.staff(tenant_id, id);

		COMMENT ON COLUMN schedule.student_pickup_exceptions.excused_from IS
			'Wall-clock time from which this child is excused on exception_date. NULL means this is only a pickup override.';
		COMMENT ON COLUMN schedule.student_pickup_exceptions.excused_owns_pickup_time IS
			'True when the partial absence created the pickup_time and may restore it on update/delete.';

		ALTER TABLE schedule.instance_students
			ADD COLUMN IF NOT EXISTS pickup_exception_id BIGINT;
		CREATE INDEX IF NOT EXISTS idx_instance_students_pickup_exception
			ON schedule.instance_students (pickup_exception_id)
			WHERE pickup_exception_id IS NOT NULL;
		CREATE UNIQUE INDEX IF NOT EXISTS uq_student_pickup_exceptions_tenant_id_id
			ON schedule.student_pickup_exceptions (tenant_id, id);

		ALTER TABLE schedule.instance_students
			DROP CONSTRAINT IF EXISTS fk_instance_students_pickup_exception;
		ALTER TABLE schedule.instance_students
			ADD CONSTRAINT fk_instance_students_pickup_exception
			FOREIGN KEY (tenant_id, pickup_exception_id)
			REFERENCES schedule.student_pickup_exceptions(tenant_id, id)
			ON DELETE SET NULL (pickup_exception_id);

		COMMENT ON COLUMN schedule.instance_students.pickup_exception_id IS
			'Identifies the pickup exception whose partial excusal set this slot absent. NULL for actual or manual decisions.';
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding partial student absence fields: %w", err)
	}
	return nil
}

func partialStudentAbsencesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.283: Removing partial student absence fields...")

	_, err := db.NewRaw(`
		DROP INDEX IF EXISTS schedule.idx_instance_students_pickup_exception;
		ALTER TABLE schedule.instance_students
			DROP CONSTRAINT IF EXISTS fk_instance_students_pickup_exception,
			DROP COLUMN IF EXISTS pickup_exception_id;
		DROP INDEX IF EXISTS schedule.uq_student_pickup_exceptions_tenant_id_id;

		ALTER TABLE schedule.student_pickup_exceptions
			DROP CONSTRAINT IF EXISTS fk_pickup_exception_excused_created_by_tenant,
			DROP CONSTRAINT IF EXISTS chk_pickup_exception_partial_absence,
			DROP COLUMN IF EXISTS excused_owns_pickup_time,
			DROP COLUMN IF EXISTS excused_created_by,
			DROP COLUMN IF EXISTS excused_reason,
			DROP COLUMN IF EXISTS excused_from;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed removing partial student absence fields: %w", err)
	}
	return nil
}
