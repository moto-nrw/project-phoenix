package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	dropMFARecoveryCodesVersion     = "1.15.87"
	dropMFARecoveryCodesDescription = "Drop auth.mfa_recovery_codes + platform.operator_mfa_recovery_codes — feature retired due to poor UX (#1308)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     dropMFARecoveryCodesVersion,
		Description: dropMFARecoveryCodesDescription,
		DependsOn:   []string{"1.15.86"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.87: Dropping auth.mfa_recovery_codes + platform.operator_mfa_recovery_codes...")
			_, err := db.ExecContext(ctx, `
				DROP TRIGGER IF EXISTS update_mfa_recovery_codes_updated_at ON auth.mfa_recovery_codes;
				DROP TABLE IF EXISTS auth.mfa_recovery_codes CASCADE;

				DROP TRIGGER IF EXISTS update_operator_mfa_recovery_codes_updated_at ON platform.operator_mfa_recovery_codes;
				DROP TABLE IF EXISTS platform.operator_mfa_recovery_codes CASCADE;
			`)
			if err != nil {
				return fmt.Errorf("drop recovery code tables: %w", err)
			}
			return nil
		},
		// Down: recreate the original DDL from migrations 1.15.80 + 1.15.83
		// so `migrate reset` (and any test harness that walks down then up)
		// can replay cleanly. Mirrors the original schema bit-for-bit.
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.87: Recreating recovery code tables...")
			_, err := db.ExecContext(ctx, `
				CREATE TABLE IF NOT EXISTS auth.mfa_recovery_codes (
					id         BIGSERIAL PRIMARY KEY,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					account_id BIGINT NOT NULL,
					code_hash  TEXT NOT NULL,
					used_at    TIMESTAMPTZ,
					CONSTRAINT fk_mfa_recovery_codes_account
						FOREIGN KEY (account_id) REFERENCES auth.accounts(id) ON DELETE CASCADE,
					CONSTRAINT uniq_mfa_recovery_codes_account_hash UNIQUE (account_id, code_hash)
				);
				CREATE INDEX IF NOT EXISTS idx_mfa_recovery_codes_account_unused
					ON auth.mfa_recovery_codes (account_id)
					WHERE used_at IS NULL;
				DROP TRIGGER IF EXISTS update_mfa_recovery_codes_updated_at ON auth.mfa_recovery_codes;
				CREATE TRIGGER update_mfa_recovery_codes_updated_at
				BEFORE UPDATE ON auth.mfa_recovery_codes
				FOR EACH ROW EXECUTE FUNCTION update_modified_column();

				CREATE TABLE IF NOT EXISTS platform.operator_mfa_recovery_codes (
					id          BIGSERIAL PRIMARY KEY,
					created_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					updated_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					operator_id BIGINT NOT NULL,
					code_hash   TEXT NOT NULL,
					used_at     TIMESTAMPTZ,
					CONSTRAINT fk_operator_mfa_recovery_codes_operator
						FOREIGN KEY (operator_id) REFERENCES platform.operators(id) ON DELETE CASCADE,
					CONSTRAINT uniq_operator_mfa_recovery_codes_op_hash UNIQUE (operator_id, code_hash)
				);
				CREATE INDEX IF NOT EXISTS idx_operator_mfa_recovery_codes_operator_unused
					ON platform.operator_mfa_recovery_codes (operator_id)
					WHERE used_at IS NULL;
				DROP TRIGGER IF EXISTS update_operator_mfa_recovery_codes_updated_at ON platform.operator_mfa_recovery_codes;
				CREATE TRIGGER update_operator_mfa_recovery_codes_updated_at
				BEFORE UPDATE ON platform.operator_mfa_recovery_codes
				FOR EACH ROW EXECUTE FUNCTION update_modified_column();
			`)
			if err != nil {
				return fmt.Errorf("recreate recovery code tables: %w", err)
			}
			return nil
		},
	)
}
