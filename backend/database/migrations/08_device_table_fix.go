package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	// Migration 8: Fix device structure
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 8: Fixing device table structure...")

			// Drop and recreate devices table with correct structure
			_, err := db.ExecContext(ctx, `
				DROP TABLE IF EXISTS devices CASCADE;
				
				CREATE TABLE IF NOT EXISTS devices (
					id SERIAL PRIMARY KEY,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					user_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
					device_id TEXT NOT NULL UNIQUE
				);
				
				CREATE INDEX IF NOT EXISTS idx_devices_device_id ON devices(device_id);
			`)

			return err
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 8: Reverting device table...")

			// Recreate the old style devices table
			_, err := db.ExecContext(ctx, `
				DROP TABLE IF EXISTS devices CASCADE;
				
				CREATE TABLE IF NOT EXISTS devices (
					id SERIAL PRIMARY KEY,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					modified_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					name TEXT NOT NULL,
					device_id TEXT NOT NULL UNIQUE,
					room_id INTEGER REFERENCES rooms(id) ON DELETE SET NULL,
					last_seen TIMESTAMPTZ,
					is_active BOOLEAN NOT NULL DEFAULT TRUE
				);
			`)

			return err
		},
	)
}
