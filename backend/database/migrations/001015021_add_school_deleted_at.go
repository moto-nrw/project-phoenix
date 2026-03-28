package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

const (
	addSchoolDeletedAtVersion     = "1.15.21"
	addSchoolDeletedAtDescription = "Add deleted_at column to platform.schools for soft-delete support"
)

func init() {
	MigrationRegistry[addSchoolDeletedAtVersion] = &Migration{
		Version:     addSchoolDeletedAtVersion,
		Description: addSchoolDeletedAtDescription,
		DependsOn:   []string{"1.15.20"},
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			if _, err := db.ExecContext(ctx, `
				ALTER TABLE platform.schools
				ADD COLUMN deleted_at TIMESTAMPTZ
			`); err != nil {
				return err
			}
			_, err := db.ExecContext(ctx, `
				CREATE INDEX idx_schools_active_visible
				ON platform.schools (id)
				WHERE active = true AND hidden = false AND deleted_at IS NULL
			`)
			return err
		},
		func(ctx context.Context, db *bun.DB) error {
			if _, err := db.ExecContext(ctx, `
				DROP INDEX IF EXISTS platform.idx_schools_active_visible
			`); err != nil {
				return err
			}
			_, err := db.ExecContext(ctx, `
				ALTER TABLE platform.schools
				DROP COLUMN IF EXISTS deleted_at
			`)
			return err
		},
	)
}
