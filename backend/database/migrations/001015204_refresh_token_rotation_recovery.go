package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	refreshTokenRotationRecoveryVersion     = "1.15.204"
	refreshTokenRotationRecoveryDescription = "Persist refresh-token rotation handoffs so interrupted frontend responses and deployments can recover without weakening bounded replay detection (#1938)."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     refreshTokenRotationRecoveryVersion,
		Description: refreshTokenRotationRecoveryDescription,
		DependsOn:   []string{staffScheduleRotationAnchorVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.204: Adding persistent refresh-token rotation recovery (#1938)...")
			if _, err := db.NewRaw(`
				ALTER TABLE auth.tokens
					ADD COLUMN IF NOT EXISTS rotated_at TIMESTAMPTZ,
					ADD COLUMN IF NOT EXISTS replacement_token TEXT,
					ADD COLUMN IF NOT EXISTS recovery_proof_hash BYTEA;
				ALTER TABLE platform.operator_refresh_tokens
					ADD COLUMN IF NOT EXISTS rotated_at TIMESTAMPTZ,
					ADD COLUMN IF NOT EXISTS replacement_token TEXT,
					ADD COLUMN IF NOT EXISTS recovery_proof_hash BYTEA;

				ALTER TABLE auth.tokens
					DROP CONSTRAINT IF EXISTS auth_tokens_rotation_handoff_complete,
					ADD CONSTRAINT auth_tokens_rotation_handoff_complete CHECK (
						(rotated_at IS NULL AND replacement_token IS NULL)
						OR (rotated_at IS NOT NULL AND replacement_token IS NOT NULL)
					);
				ALTER TABLE platform.operator_refresh_tokens
					DROP CONSTRAINT IF EXISTS operator_refresh_tokens_rotation_handoff_complete,
					ADD CONSTRAINT operator_refresh_tokens_rotation_handoff_complete CHECK (
						(rotated_at IS NULL AND replacement_token IS NULL)
						OR (rotated_at IS NOT NULL AND replacement_token IS NOT NULL)
					);

				CREATE INDEX IF NOT EXISTS idx_tokens_rotated_at
					ON auth.tokens(rotated_at) WHERE rotated_at IS NOT NULL;
				CREATE INDEX IF NOT EXISTS idx_operator_refresh_tokens_rotated_at
					ON platform.operator_refresh_tokens(rotated_at) WHERE rotated_at IS NOT NULL;

				COMMENT ON COLUMN auth.tokens.replacement_token IS
					'Opaque successor handle retained until refresh expiry for bounded recovery and replay-family attribution; never returned by APIs or logs.';
				COMMENT ON COLUMN platform.operator_refresh_tokens.replacement_token IS
					'Opaque successor handle retained until refresh expiry for bounded recovery and replay-family attribution; never returned by APIs or logs.';
				COMMENT ON COLUMN auth.tokens.recovery_proof_hash IS
					'SHA-256 digest of the independent Auth.js recovery secret required for bounded handoff recovery; never returned by APIs or logs.';
				COMMENT ON COLUMN platform.operator_refresh_tokens.recovery_proof_hash IS
					'SHA-256 digest of the independent Auth.js recovery secret required for bounded handoff recovery; never returned by APIs or logs.';
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding refresh-token rotation recovery: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.204...")
			if _, err := db.NewRaw(`
				DROP INDEX IF EXISTS auth.idx_tokens_rotated_at;
				DROP INDEX IF EXISTS platform.idx_operator_refresh_tokens_rotated_at;
				ALTER TABLE auth.tokens
					DROP CONSTRAINT IF EXISTS auth_tokens_rotation_handoff_complete,
					DROP COLUMN IF EXISTS recovery_proof_hash,
					DROP COLUMN IF EXISTS replacement_token,
					DROP COLUMN IF EXISTS rotated_at;
				ALTER TABLE platform.operator_refresh_tokens
					DROP CONSTRAINT IF EXISTS operator_refresh_tokens_rotation_handoff_complete,
					DROP COLUMN IF EXISTS recovery_proof_hash,
					DROP COLUMN IF EXISTS replacement_token,
					DROP COLUMN IF EXISTS rotated_at;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed removing refresh-token rotation recovery: %w", err)
			}
			return nil
		},
	)
}
