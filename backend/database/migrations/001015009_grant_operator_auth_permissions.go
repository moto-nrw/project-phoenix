package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	grantOperatorAuthPermissionsVersion     = "1.15.9"
	grantOperatorAuthPermissionsDescription = "Grant platform.operators read/update permissions to phoenix_auth for operator auth flows"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     grantOperatorAuthPermissionsVersion,
		Description: grantOperatorAuthPermissionsDescription,
		DependsOn:   []string{"1.15.8"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.9: Granting operator auth permissions to phoenix_auth...")

			_, err := db.ExecContext(ctx, `
				GRANT SELECT, UPDATE ON platform.operators TO phoenix_auth;
			`)
			if err != nil {
				return fmt.Errorf("error granting operator auth permissions to phoenix_auth: %w", err)
			}

			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.9: Revoking operator auth permissions from phoenix_auth...")

			_, err := db.ExecContext(ctx, `
				REVOKE SELECT, UPDATE ON platform.operators FROM phoenix_auth;
			`)
			if err != nil {
				return fmt.Errorf("error revoking operator auth permissions from phoenix_auth: %w", err)
			}

			return nil
		},
	)
}
