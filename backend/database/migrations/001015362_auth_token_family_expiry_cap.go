package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	authTokenFamilyExpiryCapVersion     = "1.15.362"
	authTokenFamilyExpiryCapDescription = "Preserve the expiry cap of retired refresh-token families (#2952)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     authTokenFamilyExpiryCapVersion,
		Description: authTokenFamilyExpiryCapDescription,
		DependsOn:   []string{staffHRPermissionsVersion},
	})
	Migrations.MustRegister(authTokenFamilyExpiryCapUp, authTokenFamilyExpiryCapDown)
}

func authTokenFamilyExpiryCapUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.NewRaw(`
		ALTER TABLE auth.tokens
			ADD COLUMN IF NOT EXISTS family_expiry_cap TIMESTAMPTZ;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("add refresh-token family expiry cap: %w", err)
	}
	return nil
}

func authTokenFamilyExpiryCapDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.NewRaw(`
		ALTER TABLE auth.tokens
			DROP COLUMN IF EXISTS family_expiry_cap;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("drop refresh-token family expiry cap: %w", err)
	}
	return nil
}
