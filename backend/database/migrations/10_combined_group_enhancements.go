package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	// Migration 10: Enhance CombinedGroup table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 10: Enhancing CombinedGroup table...")

			_, err := db.ExecContext(ctx, `
				ALTER TABLE combined_groups
				ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT TRUE,
				ADD COLUMN IF NOT EXISTS valid_until TIMESTAMPTZ,
				ADD COLUMN IF NOT EXISTS access_policy TEXT NOT NULL DEFAULT 'restricted',
				ADD COLUMN IF NOT EXISTS specific_group_id INTEGER REFERENCES groups(id) ON DELETE SET NULL;
			`)

			return err
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 10: Reverting CombinedGroup enhancements...")

			_, err := db.ExecContext(ctx, `
				ALTER TABLE combined_groups
				DROP COLUMN IF EXISTS is_active,
				DROP COLUMN IF EXISTS valid_until,
				DROP COLUMN IF EXISTS access_policy,
				DROP COLUMN IF EXISTS specific_group_id;
			`)

			return err
		},
	)
}
