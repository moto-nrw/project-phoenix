package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	dailyCheckoutAllRoomsExistingTenantsVersion     = "1.15.321"
	dailyCheckoutAllRoomsExistingTenantsDescription = "Preserve the existing daily-checkout room policy for current tenants"
	dailyCheckoutAllRoomsSettingKey                 = "checkout.daily_checkout_from_all_rooms_enabled"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     dailyCheckoutAllRoomsExistingTenantsVersion,
		Description: dailyCheckoutAllRoomsExistingTenantsDescription,
		DependsOn:   []string{configSettingValuesVersion},
	})

	Migrations.MustRegister(dailyCheckoutAllRoomsExistingTenantsUp, dailyCheckoutAllRoomsExistingTenantsDown)
}

func dailyCheckoutAllRoomsExistingTenantsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.321: Preserving the daily-checkout room policy for existing tenants...")

	if _, err := db.NewRaw(`
		WITH inserted AS (
			INSERT INTO config.setting_values (tenant_id, setting_key, value)
			SELECT id, ?, 'false'::jsonb
			FROM platform.schools
			ON CONFLICT (tenant_id, setting_key) DO NOTHING
			RETURNING tenant_id, setting_key, value
		)
		INSERT INTO config.setting_audit (
			tenant_id, setting_key, old_value, new_value, action, changed_by
		)
		SELECT tenant_id, setting_key, NULL, value, 'set', NULL
		FROM inserted;
	`, dailyCheckoutAllRoomsSettingKey).Exec(ctx); err != nil {
		return fmt.Errorf("failed backfilling daily-checkout room policy: %w", err)
	}

	return nil
}

// Stored values may have been changed after rollout and carry no migration
// provenance. Old binaries safely ignore the unknown key, so rollback is a no-op.
func dailyCheckoutAllRoomsExistingTenantsDown(_ context.Context, _ *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.321: keeping daily-checkout room policy values...")
	return nil
}
