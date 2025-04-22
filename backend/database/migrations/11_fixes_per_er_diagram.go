package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	// Migration 11: Fix missing constraints to match ER diagram
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 11: Adding missing constraints from ER diagram...")

			// 1. Add UNIQUE constraint and INDEX to groups.name
			_, err := db.ExecContext(ctx, `
				-- Add unique constraint to group names
				ALTER TABLE groups ADD CONSTRAINT IF NOT EXISTS groups_name_key UNIQUE (name);
				
				-- Create index on groups.name
				CREATE INDEX IF NOT EXISTS idx_groups_name ON groups(name);
			`)
			if err != nil {
				return fmt.Errorf("error adding constraints to groups: %w", err)
			}

			// 2. Update room_occupancy.device_id to be NOT NULL, UNIQUE and INDEXED
			_, err = db.ExecContext(ctx, `
				-- First update any NULL values with a placeholder
				UPDATE room_occupancy SET device_id = 'device_' || id WHERE device_id IS NULL;
						
				-- Make device_id NOT NULL
				ALTER TABLE room_occupancy ALTER COLUMN device_id SET NOT NULL;
				
				-- Add unique constraint
				ALTER TABLE room_occupancy ADD CONSTRAINT IF NOT EXISTS room_occupancy_device_id_key UNIQUE (device_id);
				
				-- Create index on device_id
				CREATE INDEX IF NOT EXISTS idx_room_occupancy_device_id ON room_occupancy(device_id);
			`)

			return err
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 11: Removing constraints...")

			// Remove the index and constraint from groups.name
			_, err := db.ExecContext(ctx, `
				DROP INDEX IF EXISTS idx_groups_name;
				ALTER TABLE groups DROP CONSTRAINT IF EXISTS groups_name_key;
			`)
			if err != nil {
				return err
			}

			// Remove the index and constraint from room_occupancy.device_id
			// and make it nullable again
			_, err = db.ExecContext(ctx, `
				DROP INDEX IF EXISTS idx_room_occupancy_device_id;
				ALTER TABLE room_occupancy DROP CONSTRAINT IF EXISTS room_occupancy_device_id_key;
				ALTER TABLE room_occupancy ALTER COLUMN device_id DROP NOT NULL;
			`)

			return err
		},
	)
}
