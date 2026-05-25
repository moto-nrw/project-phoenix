package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	phaseRolloverColumnsVersion     = "1.15.72"
	phaseRolloverColumnsDescription = "Add rollover columns to enrollment.phases — links a new phase back to its source phase, picks opt-in/opt-out renewal mode, sets the parent-action deadline, and toggles whether grade levels bump (yearly vs half-year cadence)."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     phaseRolloverColumnsVersion,
		Description: phaseRolloverColumnsDescription,
		DependsOn: []string{
			createEnrollmentPhasesVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.72: Adding rollover columns to enrollment.phases...")
			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.phases
				ADD COLUMN IF NOT EXISTS rollover_source_phase_id BIGINT
					REFERENCES enrollment.phases(id) ON DELETE SET NULL,
				ADD COLUMN IF NOT EXISTS rollover_mode TEXT
					CHECK (rollover_mode IS NULL OR rollover_mode IN ('opt_in','opt_out')),
				ADD COLUMN IF NOT EXISTS rollover_auto_approve BOOLEAN NOT NULL DEFAULT false,
				ADD COLUMN IF NOT EXISTS rollover_deadline TIMESTAMPTZ,
				ADD COLUMN IF NOT EXISTS rollover_bumps_grade BOOLEAN NOT NULL DEFAULT true;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding rollover columns to enrollment.phases: %w", err)
			}
			if _, err := db.NewRaw(`
				CREATE INDEX IF NOT EXISTS idx_enrollment_phases_rollover_source
				ON enrollment.phases (rollover_source_phase_id)
				WHERE rollover_source_phase_id IS NOT NULL;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed creating rollover_source index: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			if _, err := db.NewRaw(`DROP INDEX IF EXISTS enrollment.idx_enrollment_phases_rollover_source;`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping rollover index: %w", err)
			}
			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.phases
				DROP COLUMN IF EXISTS rollover_source_phase_id,
				DROP COLUMN IF EXISTS rollover_mode,
				DROP COLUMN IF EXISTS rollover_auto_approve,
				DROP COLUMN IF EXISTS rollover_deadline,
				DROP COLUMN IF EXISTS rollover_bumps_grade;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping rollover columns: %w", err)
			}
			return nil
		},
	)
}
