package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	sickAbsenceProvenanceVersion     = "1.15.198"
	sickAbsenceProvenanceDescription = "Add sick_absence_id provenance columns to schedule.staff_shifts and schedule.instance_staff (#1843): a sick report cascades into shift cancellations and block absences, and deleting the sick report must reverse exactly the rows it created — change_reason is free text and cannot carry that link."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     sickAbsenceProvenanceVersion,
		Description: sickAbsenceProvenanceDescription,
		DependsOn:   []string{careOfferingAvailabilityRulesVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.198: Adding sick_absence_id provenance columns (#1843)...")
			// No FK to active.staff_absences: the reversal runs BEFORE the
			// absence row is deleted (same tx), and a cross-schema FK would
			// couple schedule writes to the active schema for no integrity
			// gain — a dangling id after a skipped reversal is inert.
			if _, err := db.NewRaw(`
				ALTER TABLE schedule.staff_shifts
					ADD COLUMN IF NOT EXISTS sick_absence_id BIGINT;
				COMMENT ON COLUMN schedule.staff_shifts.sick_absence_id IS
					'Set when the #1843 sick-report cascade cancelled this shift; identifies exactly which active.staff_absences row caused it so deleting that report reactivates only these shifts. NULL for manual cancellations.';
				CREATE INDEX IF NOT EXISTS idx_staff_shifts_sick_absence_id
					ON schedule.staff_shifts (sick_absence_id)
					WHERE sick_absence_id IS NOT NULL;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding sick_absence_id to schedule.staff_shifts: %w", err)
			}
			if _, err := db.NewRaw(`
				ALTER TABLE schedule.instance_staff
					ADD COLUMN IF NOT EXISTS sick_absence_id BIGINT;
				COMMENT ON COLUMN schedule.instance_staff.sick_absence_id IS
					'Set when the #1843 sick-report cascade marked this row absent; identifies which active.staff_absences row caused it so deleting that report clears only these absences. NULL for manual Vertretungsplan absences.';
				CREATE INDEX IF NOT EXISTS idx_instance_staff_sick_absence_id
					ON schedule.instance_staff (sick_absence_id)
					WHERE sick_absence_id IS NOT NULL;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding sick_absence_id to schedule.instance_staff: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.198...")
			if _, err := db.NewRaw(`
				DROP INDEX IF EXISTS schedule.idx_instance_staff_sick_absence_id;
				ALTER TABLE schedule.instance_staff
					DROP COLUMN IF EXISTS sick_absence_id;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping sick_absence_id from schedule.instance_staff: %w", err)
			}
			if _, err := db.NewRaw(`
				DROP INDEX IF EXISTS schedule.idx_staff_shifts_sick_absence_id;
				ALTER TABLE schedule.staff_shifts
					DROP COLUMN IF EXISTS sick_absence_id;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping sick_absence_id from schedule.staff_shifts: %w", err)
			}
			return nil
		},
	)
}
