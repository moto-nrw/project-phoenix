package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	addMFAAdminOverrideVersion     = "1.15.88"
	addMFAAdminOverrideDescription = "Add auth.accounts.mfa_admin_override for per-user MFA opt-out/in (#1308 follow-up)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     addMFAAdminOverrideVersion,
		Description: addMFAAdminOverrideDescription,
		DependsOn:   []string{"1.15.87"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.88: Adding auth.accounts.mfa_admin_override...")
			_, err := db.ExecContext(ctx, `
				ALTER TABLE auth.accounts
				ADD COLUMN IF NOT EXISTS mfa_admin_override TEXT NOT NULL DEFAULT 'none';

				ALTER TABLE auth.accounts
				DROP CONSTRAINT IF EXISTS chk_accounts_mfa_admin_override;
				ALTER TABLE auth.accounts
				ADD CONSTRAINT chk_accounts_mfa_admin_override
				CHECK (mfa_admin_override IN ('none', 'force_off', 'force_on'));
			`)
			if err != nil {
				return fmt.Errorf("add mfa_admin_override column: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.88: Dropping mfa_admin_override column...")
			_, err := db.ExecContext(ctx, `
				ALTER TABLE auth.accounts DROP CONSTRAINT IF EXISTS chk_accounts_mfa_admin_override;
				ALTER TABLE auth.accounts DROP COLUMN IF EXISTS mfa_admin_override;
			`)
			if err != nil {
				return fmt.Errorf("drop mfa_admin_override column: %w", err)
			}
			return nil
		},
	)
}
