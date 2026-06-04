package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	makeTimetableEnabledOptOutVersion     = "1.15.98"
	makeTimetableEnabledOptOutDescription = "Make timetable feature opt-out by removing stale false overrides."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     makeTimetableEnabledOptOutVersion,
		Description: makeTimetableEnabledOptOutDescription,
		DependsOn:   []string{configSettingValuesVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.98: Making timetable.enabled opt-out...")

			if _, err := db.NewRaw(`
				WITH deleted AS (
					DELETE FROM config.setting_values
					WHERE setting_key = 'timetable.enabled'
						AND value = 'false'::jsonb
					RETURNING tenant_id, setting_key, value
				)
				INSERT INTO config.setting_audit (
					tenant_id,
					setting_key,
					old_value,
					new_value,
					action,
					changed_by
				)
				SELECT
					tenant_id,
					setting_key,
					value,
					NULL,
					'reset',
					NULL
				FROM deleted;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed resetting stale timetable.enabled=false overrides: %w", err)
			}

			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.98: no-op for timetable opt-out reset...")
			return nil
		},
	)
}
