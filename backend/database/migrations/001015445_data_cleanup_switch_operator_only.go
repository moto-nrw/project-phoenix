package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	dataCleanupSwitchOperatorOnlyVersion     = "1.15.445"
	dataCleanupSwitchOperatorOnlyDescription = "Nächtliche Löschung: von Schulen gesetzte Abschaltungen zurücksetzen (#3951, ADR 0048)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     dataCleanupSwitchOperatorOnlyVersion,
		Description: dataCleanupSwitchOperatorOnlyDescription,
		DependsOn:   []string{cleanupRunsVersion, configSettingValuesVersion},
	})

	Migrations.MustRegister(dataCleanupSwitchOperatorOnlyUp, dataCleanupSwitchOperatorOnlyDown)
}

// dataCleanupSwitchOperatorOnlyUp removes every stored "off" of the nightly
// deletion. Since #3951 only the moto team may halt it (ADR 0048); a school
// that switched it off earlier could no longer see the switch, and its
// stored value would keep breaking the retention the contract promises.
// Each reset is recorded in the settings audit like a manual reset by the
// system; the release is the announced cut-over date for existing schools.
func dataCleanupSwitchOperatorOnlyUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.NewRaw(`
		WITH deleted AS (
			DELETE FROM config.setting_values
			WHERE setting_key = 'gdpr.data_cleanup_enabled'
			  AND value = 'false'::jsonb
			RETURNING tenant_id, setting_key, value
		)
		INSERT INTO config.setting_audit (tenant_id, setting_key, old_value, new_value, action, changed_by)
		SELECT tenant_id, setting_key, value, NULL, 'reset', NULL
		FROM deleted;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("reset gdpr.data_cleanup_enabled=false overrides: %w", err)
	}
	return nil
}

// dataCleanupSwitchOperatorOnlyDown keeps the reset: restoring an "off" would
// silently stop the deletion again. The settings audit names every school
// whose value went, so the moto team can restore one deliberately.
func dataCleanupSwitchOperatorOnlyDown(context.Context, *bun.DB) error {
	return nil
}
