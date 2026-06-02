package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	phaseCareOfferingSelectionModeVersion     = "1.15.97"
	phaseCareOfferingSelectionModeDescription = "Add per-phase care offering selection mode for optional, at-least-one, or exactly-one parent choices."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     phaseCareOfferingSelectionModeVersion,
		Description: phaseCareOfferingSelectionModeDescription,
		DependsOn:   []string{createEnrollmentPhasesVersion, configSettingValuesVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.97: Adding care_offering_selection_mode to enrollment.phases...")
			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.phases
				ADD COLUMN IF NOT EXISTS care_offering_selection_mode TEXT NOT NULL DEFAULT 'optional';

				ALTER TABLE enrollment.phases
				DROP CONSTRAINT IF EXISTS chk_enrollment_phases_care_offering_selection_mode;

				ALTER TABLE enrollment.phases
				ADD CONSTRAINT chk_enrollment_phases_care_offering_selection_mode
				CHECK (care_offering_selection_mode IN ('optional', 'at_least_one', 'exactly_one'));
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding care_offering_selection_mode: %w", err)
			}

			if _, err := db.NewRaw(`
				UPDATE enrollment.phases AS p
				SET care_offering_selection_mode = 'at_least_one'
				FROM config.setting_values AS sv
				WHERE sv.tenant_id = p.tenant_id
					AND sv.setting_key = 'enrollment.care_offerings_required'
					AND sv.value = 'true'::jsonb;

				DELETE FROM config.setting_values
				WHERE setting_key = 'enrollment.care_offerings_required';

				DELETE FROM config.setting_audit
				WHERE setting_key = 'enrollment.care_offerings_required';
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed backfilling care_offering_selection_mode: %w", err)
			}

			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.97...")
			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.phases
				DROP CONSTRAINT IF EXISTS chk_enrollment_phases_care_offering_selection_mode;

				ALTER TABLE enrollment.phases
				DROP COLUMN IF EXISTS care_offering_selection_mode;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping care_offering_selection_mode: %w", err)
			}
			return nil
		},
	)
}
