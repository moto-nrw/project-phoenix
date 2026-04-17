package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

const (
	addOrganizationDeletedAtVersion     = "1.15.32"
	addOrganizationDeletedAtDescription = "Add deleted_at column to platform.organizations for soft-delete support"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     addOrganizationDeletedAtVersion,
		Description: addOrganizationDeletedAtDescription,
		DependsOn:   []string{"1.15.31"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			_, err := db.ExecContext(ctx, `
				ALTER TABLE platform.organizations
				ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ
			`)
			return err
		},
		func(ctx context.Context, db *bun.DB) error {
			_, err := db.ExecContext(ctx, `
				ALTER TABLE platform.organizations
				DROP COLUMN IF EXISTS deleted_at
			`)
			return err
		},
	)
}
