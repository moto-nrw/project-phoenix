package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	MigrationRegistry.Register(&Migration{Version: "1.15.480", Description: "Persist second-factor proof across account session rotation (#3841-f)", DependsOn: []string{"1.15.479"}})
	Migrations.MustRegister(sessionMFAProofUp, sessionMFAProofDown)
}

func sessionMFAProofUp(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `ALTER TABLE auth.tokens ADD COLUMN mfa_verified BOOLEAN NOT NULL DEFAULT FALSE`)
	return err
}

func sessionMFAProofDown(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `ALTER TABLE auth.tokens DROP COLUMN mfa_verified`)
	return err
}
