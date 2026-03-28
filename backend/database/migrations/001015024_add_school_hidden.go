package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

const (
	addSchoolHiddenVersion     = "1.15.24"
	addSchoolHiddenDescription = "Add hidden column to platform.schools for landing page visibility control"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     addSchoolHiddenVersion,
		Description: addSchoolHiddenDescription,
		DependsOn:   []string{"1.15.23"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			_, err := db.ExecContext(ctx, `
				ALTER TABLE platform.schools
				ADD COLUMN IF NOT EXISTS hidden BOOLEAN NOT NULL DEFAULT FALSE
			`)
			return err
		},
		func(ctx context.Context, db *bun.DB) error {
			_, err := db.ExecContext(ctx, `
				ALTER TABLE platform.schools
				DROP COLUMN IF EXISTS hidden
			`)
			return err
		},
	)
}
