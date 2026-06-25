package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	operatorRefreshTokensVersion     = "1.15.147"
	operatorRefreshTokensDescription = "Persist revocable operator refresh sessions so password rotation and token replay can invalidate platform sessions."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     operatorRefreshTokensVersion,
		Description: operatorRefreshTokensDescription,
		DependsOn:   []string{guardianHasAccountCheckVersion},
	})

	Migrations.MustRegister(upOperatorRefreshTokens, downOperatorRefreshTokens)
}

func upOperatorRefreshTokens(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.147: Creating platform.operator_refresh_tokens...")
	if _, err := db.NewRaw(`
		CREATE TABLE IF NOT EXISTS platform.operator_refresh_tokens (
			id BIGSERIAL PRIMARY KEY,
			operator_id BIGINT NOT NULL REFERENCES platform.operators(id) ON DELETE CASCADE,
			token TEXT NOT NULL UNIQUE,
			expiry TIMESTAMPTZ NOT NULL,
			family_id TEXT NOT NULL,
			generation INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_operator_refresh_tokens_operator_id
			ON platform.operator_refresh_tokens(operator_id);
		CREATE INDEX IF NOT EXISTS idx_operator_refresh_tokens_expiry
			ON platform.operator_refresh_tokens(expiry);
		CREATE INDEX IF NOT EXISTS idx_operator_refresh_tokens_family_id
			ON platform.operator_refresh_tokens(family_id);

		DROP TRIGGER IF EXISTS update_operator_refresh_tokens_updated_at
			ON platform.operator_refresh_tokens;
		CREATE TRIGGER update_operator_refresh_tokens_updated_at
			BEFORE UPDATE ON platform.operator_refresh_tokens
			FOR EACH ROW
			EXECUTE FUNCTION update_modified_column();

		GRANT SELECT, INSERT, UPDATE, DELETE ON platform.operator_refresh_tokens TO phoenix_auth;
		GRANT USAGE, SELECT ON SEQUENCE platform.operator_refresh_tokens_id_seq TO phoenix_auth;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating operator refresh tokens: %w", err)
	}
	return nil
}

func downOperatorRefreshTokens(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.147: Dropping platform.operator_refresh_tokens...")
	if _, err := db.NewRaw(`
		DROP TRIGGER IF EXISTS update_operator_refresh_tokens_updated_at
			ON platform.operator_refresh_tokens;
		DROP TABLE IF EXISTS platform.operator_refresh_tokens CASCADE;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping operator refresh tokens: %w", err)
	}
	return nil
}
