package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	permissionCatalogIsolationVersion     = "1.15.323"
	permissionCatalogIsolationDescription = "Restrict global permission catalog writes to platform database role"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     permissionCatalogIsolationVersion,
		Description: permissionCatalogIsolationDescription,
		DependsOn:   []string{"1.14.1"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.323: Restricting global permission catalog writes...")
			if _, err := db.ExecContext(ctx,
				`REVOKE INSERT, UPDATE, DELETE ON auth.permissions FROM phoenix_tenant;`,
			); err != nil {
				return fmt.Errorf("revoke tenant writes on auth.permissions: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.323: Restoring tenant permission catalog writes...")
			if _, err := db.ExecContext(ctx,
				`GRANT INSERT, UPDATE, DELETE ON auth.permissions TO phoenix_tenant;`,
			); err != nil {
				return fmt.Errorf("restore tenant writes on auth.permissions: %w", err)
			}
			return nil
		},
	)
}
