package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	// Migration 9: Add missing room_occupancy fields
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 9: Adding missing room_occupancy fields...")

			// Add missing fields to room_occupancy
			_, err := db.ExecContext(ctx, `
				ALTER TABLE room_occupancy
				ADD COLUMN IF NOT EXISTS device_id TEXT,
				ADD COLUMN IF NOT EXISTS ag_id INTEGER REFERENCES ags(id) ON DELETE SET NULL,
				ADD COLUMN IF NOT EXISTS group_id INTEGER REFERENCES groups(id) ON DELETE SET NULL;
			`)

			return err
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 9: Removing room_occupancy fields...")

			// Remove added fields from room_occupancy
			_, err := db.ExecContext(ctx, `
				ALTER TABLE room_occupancy
				DROP COLUMN IF EXISTS device_id,
				DROP COLUMN IF EXISTS ag_id,
				DROP COLUMN IF EXISTS group_id;
			`)

			return err
		},
	)
}
