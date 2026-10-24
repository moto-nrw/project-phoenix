package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	MigrationRegistry.Register(&Migration{Version: "1.15.499", Description: "Account password login protection and reset-bound MFA proof (#3841-c)", DependsOn: []string{"1.15.491"}})
	Migrations.MustRegister(passwordLoginProtectionUp, passwordLoginProtectionDown)
}

func passwordLoginProtectionUp(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		ALTER TABLE auth.accounts
			ADD COLUMN password_attempts INTEGER NOT NULL DEFAULT 0 CHECK (password_attempts >= 0),
			ADD COLUMN password_locked_until TIMESTAMPTZ,
			ADD COLUMN mfa_reset_generation BIGINT NOT NULL DEFAULT 0;
		ALTER TABLE platform.operators
			ADD COLUMN password_attempts INTEGER NOT NULL DEFAULT 0 CHECK (password_attempts >= 0),
			ADD COLUMN password_locked_until TIMESTAMPTZ;
		ALTER TABLE auth.mfa_email_challenges ADD COLUMN mfa_reset_generation BIGINT NOT NULL DEFAULT 0;
	`)
	return err
}

func passwordLoginProtectionDown(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		ALTER TABLE auth.mfa_email_challenges DROP COLUMN mfa_reset_generation;
		ALTER TABLE platform.operators DROP COLUMN password_attempts, DROP COLUMN password_locked_until;
		ALTER TABLE auth.accounts DROP COLUMN password_attempts, DROP COLUMN password_locked_until, DROP COLUMN mfa_reset_generation;
	`)
	return err
}
