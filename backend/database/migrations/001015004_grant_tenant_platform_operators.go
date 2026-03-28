package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	grantTenantPlatformOperatorsVersion     = "1.15.4"
	grantTenantPlatformOperatorsDescription = "Grant SELECT on platform.operators to phoenix_tenant (needed for suggestions comment author resolution)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     grantTenantPlatformOperatorsVersion,
		Description: grantTenantPlatformOperatorsDescription,
		DependsOn:   []string{"1.15.3"}, // after drop defaults
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.4: Granting SELECT on platform.operators to phoenix_tenant...")

			_, err := db.ExecContext(ctx,
				`GRANT SELECT ON platform.operators TO phoenix_tenant;`)
			if err != nil {
				return fmt.Errorf("error granting SELECT on platform.operators to phoenix_tenant: %w", err)
			}

			fmt.Println("Migration 1.15.4: Done")
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.4: Revoking SELECT on platform.operators from phoenix_tenant...")

			_, err := db.ExecContext(ctx,
				`REVOKE SELECT ON platform.operators FROM phoenix_tenant;`)
			if err != nil {
				return fmt.Errorf("error revoking SELECT on platform.operators from phoenix_tenant: %w", err)
			}

			fmt.Println("Migration 1.15.4: Rollback complete")
			return nil
		},
	)
}
