package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	dropMFAEmailResendCooldownSettingVersion     = "1.15.57"
	dropMFAEmailResendCooldownSettingDescription = "Delete orphaned config rows for retired security.mfa_email_resend_cooldown_seconds setting"
	dropMFAEmailResendCooldownSettingKey         = "security.mfa_email_resend_cooldown_seconds"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     dropMFAEmailResendCooldownSettingVersion,
		Description: dropMFAEmailResendCooldownSettingDescription,
		DependsOn:   []string{"1.15.56"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.57: Deleting orphaned setting rows for security.mfa_email_resend_cooldown_seconds...")

			// Drop tenant overrides first so the audit row reflects the final
			// state. Both queries use the superuser connection, which bypasses
			// RLS on config.setting_values + config.setting_audit.
			if _, err := db.ExecContext(ctx,
				`DELETE FROM config.setting_values WHERE setting_key = ?`,
				dropMFAEmailResendCooldownSettingKey,
			); err != nil {
				return fmt.Errorf("delete setting_values: %w", err)
			}
			if _, err := db.ExecContext(ctx,
				`DELETE FROM config.setting_audit WHERE setting_key = ?`,
				dropMFAEmailResendCooldownSettingKey,
			); err != nil {
				return fmt.Errorf("delete setting_audit: %w", err)
			}

			fmt.Println("Migration 1.15.57: Done")
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			// Irreversible: the registry definition is gone, so there's
			// nothing meaningful to restore. The down migration is a no-op so
			// `migrate reset` still works.
			fmt.Println("Rolling back migration 1.15.57: no-op (orphaned rows cannot be restored)")
			return nil
		},
	)
}
