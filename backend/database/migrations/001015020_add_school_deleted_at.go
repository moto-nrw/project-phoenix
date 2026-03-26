package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

const (
	addSchoolDeletedAtVersion     = "1.15.20"
	addSchoolDeletedAtDescription = "Add deleted_at column to platform.schools for soft-delete support"
)

func init() {
	MigrationRegistry[addSchoolDeletedAtVersion] = &Migration{
		Version:     addSchoolDeletedAtVersion,
		Description: addSchoolDeletedAtDescription,
		DependsOn:   []string{"1.15.19"},
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			_, err := db.ExecContext(ctx, `
				ALTER TABLE platform.schools
				ADD COLUMN deleted_at TIMESTAMPTZ
			`)
			return err
		},
		func(ctx context.Context, db *bun.DB) error {
			_, err := db.ExecContext(ctx, `
				ALTER TABLE platform.schools
				DROP COLUMN IF EXISTS deleted_at
			`)
			return err
		},
	)
}
