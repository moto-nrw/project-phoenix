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
		INSERT INTO config.setting_values (tenant_id, setting_key, value)
		SELECT id, ?, 'false'::jsonb
		FROM platform.schools
		ON CONFLICT (tenant_id, setting_key) DO NOTHING;
	`, dailyCheckoutAllRoomsSettingKey).Exec(ctx); err != nil {
		return fmt.Errorf("failed backfilling daily-checkout room policy: %w", err)
	}

	return nil
}

func dailyCheckoutAllRoomsExistingTenantsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.321: Removing the daily-checkout room policy overrides...")

	if _, err := db.NewRaw(`
		DELETE FROM config.setting_audit WHERE setting_key = ?;
		DELETE FROM config.setting_values WHERE setting_key = ?;
	`, dailyCheckoutAllRoomsSettingKey, dailyCheckoutAllRoomsSettingKey).Exec(ctx); err != nil {
		return fmt.Errorf("failed removing daily-checkout room policy overrides: %w", err)
	}

	return nil
}
