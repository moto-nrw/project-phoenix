package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	mfaTrustedDevicesTenantScopeVersion     = "1.15.89"
	mfaTrustedDevicesTenantScopeDescription = "Bind auth.mfa_trusted_devices to (account_id, tenant_id) so a cookie issued in one tenant cannot bypass MFA in another (#1308 review item #9)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     mfaTrustedDevicesTenantScopeVersion,
		Description: mfaTrustedDevicesTenantScopeDescription,
		DependsOn:   []string{"1.15.88"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.89: adding tenant_id to auth.mfa_trusted_devices...")
			_, err := db.ExecContext(ctx, `
				-- 2FA hasn't shipped yet, so existing rows are throwaway. Clearing
				-- them is the only way to add a NOT NULL column without a sentinel
				-- and avoids retroactively pinning historical cookies to a guessed
				-- tenant. After this migration users re-trust their devices on the
				-- next login if "remember device" was active.
				DELETE FROM auth.mfa_trusted_devices;

				ALTER TABLE auth.mfa_trusted_devices
				ADD COLUMN tenant_id BIGINT NOT NULL
					REFERENCES platform.schools(id) ON DELETE CASCADE;

				-- Replace the old account-scoped uniqueness with one that also
				-- includes tenant_id, so the same raw cookie value cannot collide
				-- with a row from a different tenant.
				ALTER TABLE auth.mfa_trusted_devices
				DROP CONSTRAINT IF EXISTS uniq_mfa_trusted_devices_account_token;
				ALTER TABLE auth.mfa_trusted_devices
				ADD CONSTRAINT uniq_mfa_trusted_devices_account_tenant_token
					UNIQUE (account_id, tenant_id, token_hash);

				-- Replace the per-account active-lookup index with a
				-- per-(account, tenant) one — that's how VerifyTrustedDevice
				-- now queries.
				DROP INDEX IF EXISTS auth.idx_mfa_trusted_devices_account_active;
				CREATE INDEX IF NOT EXISTS idx_mfa_trusted_devices_account_tenant_active
					ON auth.mfa_trusted_devices (account_id, tenant_id, expires_at DESC)
					WHERE revoked_at IS NULL;
			`)
			if err != nil {
				return fmt.Errorf("add tenant_id to auth.mfa_trusted_devices: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.89: dropping tenant_id from auth.mfa_trusted_devices...")
			_, err := db.ExecContext(ctx, `
				DROP INDEX IF EXISTS auth.idx_mfa_trusted_devices_account_tenant_active;
				CREATE INDEX IF NOT EXISTS idx_mfa_trusted_devices_account_active
					ON auth.mfa_trusted_devices (account_id, expires_at DESC)
					WHERE revoked_at IS NULL;

				ALTER TABLE auth.mfa_trusted_devices
				DROP CONSTRAINT IF EXISTS uniq_mfa_trusted_devices_account_tenant_token;
				ALTER TABLE auth.mfa_trusted_devices
				ADD CONSTRAINT uniq_mfa_trusted_devices_account_token
					UNIQUE (account_id, token_hash);

				ALTER TABLE auth.mfa_trusted_devices
				DROP COLUMN IF EXISTS tenant_id;
			`)
			if err != nil {
				return fmt.Errorf("drop tenant_id from auth.mfa_trusted_devices: %w", err)
			}
			return nil
		},
	)
}
