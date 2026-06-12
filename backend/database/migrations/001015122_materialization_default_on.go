package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	materializationDefaultOnVersion     = "1.15.122"
	materializationDefaultOnDescription = "Make timetable materialization opt-out by removing stale false overrides."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     materializationDefaultOnVersion,
		Description: materializationDefaultOnDescription,
		DependsOn:   []string{configSettingValuesVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.122: Making timetable.materialization_enabled opt-out...")

			if _, err := db.NewRaw(`
				WITH deleted AS (
					DELETE FROM config.setting_values
					WHERE setting_key = 'timetable.materialization_enabled'
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
				return fmt.Errorf("failed resetting stale timetable.materialization_enabled=false overrides: %w", err)
			}

			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.122: no-op for materialization opt-out reset...")
			return nil
		},
	)
}
