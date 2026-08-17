package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	autoPartialAbsenceVersion     = "1.15.299"
	autoPartialAbsenceDescription = "Derive partial absences automatically from pulled-forward day pickup times (#2360)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     autoPartialAbsenceVersion,
		Description: autoPartialAbsenceDescription,
		DependsOn:   []string{partialStudentAbsencesVersion},
	})
	Migrations.MustRegister(autoPartialAbsenceUp, autoPartialAbsenceDown)
}

func autoPartialAbsenceUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.299: Adding excused_auto to pickup exceptions (#2360)...")

	_, err := db.NewRaw(`
		ALTER TABLE schedule.student_pickup_exceptions
			ADD COLUMN IF NOT EXISTS excused_auto BOOLEAN NOT NULL DEFAULT FALSE;

		ALTER TABLE schedule.student_pickup_exceptions
			DROP CONSTRAINT IF EXISTS chk_pickup_exception_partial_absence;
		ALTER TABLE schedule.student_pickup_exceptions
			ADD CONSTRAINT chk_pickup_exception_partial_absence CHECK (
				(excused_from IS NULL AND excused_reason IS NULL AND excused_created_by IS NULL AND NOT excused_owns_pickup_time AND NOT excused_auto)
				OR (excused_from IS NOT NULL AND excused_auto AND excused_created_by IS NULL AND NOT excused_owns_pickup_time AND pickup_time IS NOT NULL)
				OR (excused_from IS NOT NULL AND NOT excused_auto AND excused_created_by IS NOT NULL AND (NOT excused_owns_pickup_time OR pickup_time IS NOT NULL))
			);

		COMMENT ON COLUMN schedule.student_pickup_exceptions.excused_auto IS
			'True when excused_from was derived automatically from a pulled-forward pickup time (#2360). Auto rows follow the pickup time; manual partial absences (excused_auto = false) are never touched by the sync.';
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding excused_auto to pickup exceptions: %w", err)
	}
	return nil
}

func autoPartialAbsenceDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.299: Removing excused_auto from pickup exceptions...")

	_, err := db.NewRaw(`
		UPDATE schedule.instance_students AS attendance
		SET status = 'expected',
			substatus = NULL,
			pickup_exception_id = NULL
		FROM schedule.student_pickup_exceptions AS exc
		WHERE attendance.pickup_exception_id = exc.id
			AND attendance.tenant_id = exc.tenant_id
			AND exc.excused_auto;

		UPDATE schedule.student_pickup_exceptions
		SET excused_from = NULL,
			excused_reason = NULL,
			excused_created_by = NULL,
			excused_owns_pickup_time = FALSE
		WHERE excused_auto;

		ALTER TABLE schedule.student_pickup_exceptions
			DROP CONSTRAINT IF EXISTS chk_pickup_exception_partial_absence;
		ALTER TABLE schedule.student_pickup_exceptions
			ADD CONSTRAINT chk_pickup_exception_partial_absence CHECK (
				(excused_from IS NULL AND excused_reason IS NULL AND excused_created_by IS NULL AND NOT excused_owns_pickup_time)
				OR (excused_from IS NOT NULL AND excused_created_by IS NOT NULL AND (NOT excused_owns_pickup_time OR pickup_time IS NOT NULL))
			);

		ALTER TABLE schedule.student_pickup_exceptions
			DROP COLUMN IF EXISTS excused_auto;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed removing excused_auto from pickup exceptions: %w", err)
	}
	return nil
}
