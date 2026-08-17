package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
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

	// Backfill: exceptions stored BEFORE this feature whose day pickup time is
	// earlier than the weekly baseline of that weekday must couple too —
	// otherwise they stay unflagged until someone edits them. Flag them and
	// apply the block absences in one statement, mirroring the runtime rule
	// (strictly earlier than the baseline; rows with any existing partial
	// absence, manual or otherwise, are skipped) and the runtime
	// ApplyPartialAbsence predicate (never rows with recorded attendance, a
	// manual status, a status-day owner, or on cancelled instances). Today is
	// computed in Berlin time, matching timezone.TodayDate() at runtime.
	res, err := db.NewRaw(`
		WITH flagged AS (
			UPDATE schedule.student_pickup_exceptions AS exc
			SET excused_from = exc.pickup_time,
				excused_auto = TRUE
			FROM schedule.student_pickup_schedules AS weekly
			WHERE weekly.tenant_id = exc.tenant_id
				AND weekly.student_id = exc.student_id
				AND weekly.weekday = EXTRACT(ISODOW FROM exc.exception_date)::int
				AND exc.exception_date >= ?
				AND exc.pickup_time IS NOT NULL
				AND exc.excused_from IS NULL
				AND exc.pickup_time < weekly.pickup_time
			RETURNING exc.tenant_id, exc.id, exc.student_id, exc.exception_date, exc.excused_from
		)
		UPDATE schedule.instance_students AS attendance
		SET status = 'absent',
			substatus = 'excused',
			student_status_day_id = NULL,
			pickup_exception_id = flagged.id,
			updated_at = NOW()
		FROM flagged, schedule.activity_instances AS instance
		WHERE attendance.tenant_id = flagged.tenant_id
			AND attendance.student_id = flagged.student_id
			AND attendance.manual_status_at IS NULL
			AND NOT attendance.not_scheduled
			AND (
				attendance.status = 'expected'
				OR (
					attendance.status = 'absent'
					AND attendance.pickup_exception_id IS NULL
					AND attendance.student_status_day_id IS NULL
				)
			)
			AND instance.id = attendance.instance_id
			AND instance.tenant_id = attendance.tenant_id
			AND instance.date = flagged.exception_date
			AND instance.start_time >= flagged.excused_from
			AND instance.status <> 'cancelled'
	`, timezone.TodayDate()).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed backfilling auto partial absences from pickup exceptions: %w", err)
	}
	if rows, rowsErr := res.RowsAffected(); rowsErr == nil {
		fmt.Printf("Migration 1.15.299: backfilled %d attendance blocks from existing pulled-forward pickup exceptions\n", rows)
	}
	return nil
}

func autoPartialAbsenceDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.299: Removing excused_auto from pickup exceptions...")

	// Restore linked blocks with the same ownership rules as
	// ReleasePartialAbsence: a still-active full-day status takes over the
	// row, completed blocks stay absent, and only actionable blocks return to
	// expected — an unconditional reset would reopen completed blocks or
	// override an active sick/excused day.
	_, err := db.NewRaw(`
		WITH released AS (
			SELECT tenant_id, id, student_id, exception_date
			FROM schedule.student_pickup_exceptions
			WHERE excused_auto
		), replacement AS (
			SELECT released.tenant_id,
				released.id AS exception_id,
				released.student_id,
				latest.id AS status_day_id,
				latest.status AS status_day_status
			FROM released
			LEFT JOIN LATERAL (
				SELECT candidate.id, candidate.status
				FROM active.student_status_days AS candidate
				WHERE candidate.tenant_id = released.tenant_id
					AND candidate.student_id = released.student_id
					AND candidate.date = released.exception_date
					AND candidate.cleared_at IS NULL
				ORDER BY candidate.reported_at DESC, candidate.id DESC
				LIMIT 1
			) AS latest ON TRUE
		)
		UPDATE schedule.instance_students AS attendance
		SET status = CASE
				WHEN replacement.status_day_id IS NOT NULL THEN 'absent'
				WHEN instance.status = 'completed' THEN 'absent'
				ELSE 'expected'
			END,
			substatus = CASE replacement.status_day_status
				WHEN 'sick' THEN 'sick'
				WHEN 'excused' THEN 'excused'
				WHEN 'class_trip' THEN 'field_trip'
				ELSE NULL
			END,
			student_status_day_id = replacement.status_day_id,
			pickup_exception_id = NULL,
			updated_at = NOW()
		FROM schedule.activity_instances AS instance, replacement
		WHERE attendance.tenant_id = replacement.tenant_id
			AND attendance.pickup_exception_id = replacement.exception_id
			AND attendance.student_id = replacement.student_id
			AND instance.id = attendance.instance_id
			AND instance.tenant_id = attendance.tenant_id;

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
