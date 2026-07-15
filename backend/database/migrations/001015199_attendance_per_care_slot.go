package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	attendancePerCareSlotVersion     = "1.15.199"
	attendancePerCareSlotDescription = "Persist student attendance per concrete care slot, including unplanned visits and status-day provenance (#1913)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     attendancePerCareSlotVersion,
		Description: attendancePerCareSlotDescription,
		DependsOn:   []string{sickAbsenceProvenanceVersion},
	})

	Migrations.MustRegister(attendancePerCareSlotUp, attendancePerCareSlotDown)
}

func attendancePerCareSlotUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.199: Adding care-slot attendance provenance (#1913)...")

	if _, err := db.NewRaw(`
		ALTER TABLE schedule.instance_students
			ADD COLUMN IF NOT EXISTS checked_out_at TIMESTAMPTZ,
			ADD COLUMN IF NOT EXISTS is_unplanned BOOLEAN NOT NULL DEFAULT FALSE,
			ADD COLUMN IF NOT EXISTS student_status_day_id BIGINT;

		COMMENT ON COLUMN schedule.instance_students.checked_out_at IS
			'Last observed checkout from this concrete care slot. Intermediate room visits remain in active.visits.';
		COMMENT ON COLUMN schedule.instance_students.is_unplanned IS
			'True when an observed visit created this slot attendance without a booking or planned roster row.';
		COMMENT ON COLUMN schedule.instance_students.student_status_day_id IS
			'Identifies the active.student_status_days row that set this slot absent. NULL for manual slot decisions.';

		CREATE INDEX IF NOT EXISTS idx_instance_students_status_day
			ON schedule.instance_students (student_status_day_id)
			WHERE student_status_day_id IS NOT NULL;
		CREATE UNIQUE INDEX IF NOT EXISTS uq_student_status_days_tenant_id_id
			ON active.student_status_days (tenant_id, id);

		DO $$ BEGIN
			ALTER TABLE schedule.instance_students
				ADD CONSTRAINT fk_instance_students_status_day
				FOREIGN KEY (tenant_id, student_status_day_id)
				REFERENCES active.student_status_days(tenant_id, id)
				ON DELETE SET NULL (student_status_day_id);
		EXCEPTION WHEN duplicate_object THEN NULL;
		END $$;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed adding care-slot attendance columns: %w", err)
	}

	// Preserve retained visit evidence. Existing planned rows gain an
	// unambiguous last checkout; retained walk-ins become durable slot rows.
	// Two deliberate guards:
	//   * status <> 'absent' — a manual or status-day absence documented by
	//     staff must never be flipped to present by historical visit data
	//     (mirrors the runtime monotonicity rule in shouldPreserveAttendanceOnCheckin).
	//   * Berlin-day match between visit and instance — if an active group is
	//     ever bridged to more than one instance, visit bounds must not smear
	//     across dates.
	if _, err := db.NewRaw(`
		WITH visit_bounds AS (
			SELECT
				instance.id AS instance_id,
				visit.student_id,
				MIN(visit.entry_time) AS checked_in_at,
				CASE
					WHEN BOOL_OR(visit.exit_time IS NULL) THEN NULL
					ELSE MAX(visit.exit_time)
				END AS checked_out_at
			FROM active.visits AS visit
			JOIN schedule.activity_instances AS instance
				ON instance.tenant_id = visit.tenant_id
				AND instance.active_group_id = visit.active_group_id
				AND (visit.entry_time AT TIME ZONE 'Europe/Berlin')::date = instance.date
			GROUP BY instance.id, visit.student_id
		)
		UPDATE schedule.instance_students AS attendance
		SET status = 'present',
			checked_in_at = COALESCE(attendance.checked_in_at, bounds.checked_in_at),
			checked_out_at = bounds.checked_out_at
		FROM visit_bounds AS bounds
		WHERE attendance.instance_id = bounds.instance_id
			AND attendance.student_id = bounds.student_id
			AND attendance.status <> 'absent';

		WITH visit_bounds AS (
			SELECT
				instance.tenant_id,
				instance.id AS instance_id,
				visit.student_id,
				MIN(visit.entry_time) AS checked_in_at,
				CASE
					WHEN BOOL_OR(visit.exit_time IS NULL) THEN NULL
					ELSE MAX(visit.exit_time)
				END AS checked_out_at
			FROM active.visits AS visit
			JOIN schedule.activity_instances AS instance
				ON instance.tenant_id = visit.tenant_id
				AND instance.active_group_id = visit.active_group_id
				AND (visit.entry_time AT TIME ZONE 'Europe/Berlin')::date = instance.date
			GROUP BY instance.tenant_id, instance.id, visit.student_id
		)
		INSERT INTO schedule.instance_students
			(tenant_id, instance_id, student_id, status, checked_in_at, checked_out_at, is_unplanned)
		SELECT tenant_id, instance_id, student_id, 'present', checked_in_at, checked_out_at, TRUE
		FROM visit_bounds
		ON CONFLICT (instance_id, student_id) DO NOTHING;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed backfilling retained care-slot visits: %w", err)
	}

	return nil
}

func attendancePerCareSlotDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.199...")
	if _, err := db.NewRaw(`
		DROP INDEX IF EXISTS schedule.idx_instance_students_status_day;
		ALTER TABLE schedule.instance_students
			DROP CONSTRAINT IF EXISTS fk_instance_students_status_day,
			DROP COLUMN IF EXISTS student_status_day_id,
			DROP COLUMN IF EXISTS is_unplanned,
			DROP COLUMN IF EXISTS checked_out_at;
		DROP INDEX IF EXISTS active.uq_student_status_days_tenant_id_id;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed removing care-slot attendance columns: %w", err)
	}
	return nil
}
