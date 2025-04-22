package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	// Migration 5: Settings and junction tables
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 5: Creating settings and additional junction tables...")

			// Ensure required student tables exist
			if err := ensureStudentTablesExist(ctx, db); err != nil {
				return err
			}

			// Create settings table
			_, err := db.ExecContext(ctx, `
				CREATE TABLE IF NOT EXISTS settings (
					id SERIAL PRIMARY KEY,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					modified_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					key TEXT NOT NULL UNIQUE,
					value JSONB NOT NULL,
					description TEXT
				)
			`)
			if err != nil {
				return err
			}

			// Create devices table
			_, err = db.ExecContext(ctx, `
				CREATE TABLE IF NOT EXISTS devices (
					id SERIAL PRIMARY KEY,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					modified_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					name TEXT NOT NULL,
					device_id TEXT NOT NULL UNIQUE,
					room_id INTEGER REFERENCES rooms(id) ON DELETE SET NULL,
					last_seen TIMESTAMPTZ,
					is_active BOOLEAN NOT NULL DEFAULT TRUE
				)
			`)
			if err != nil {
				return err
			}

			// Create combined_groups table
			_, err = db.ExecContext(ctx, `
				CREATE TABLE IF NOT EXISTS combined_groups (
					id SERIAL PRIMARY KEY,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					modified_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					name TEXT NOT NULL UNIQUE,
					description TEXT
				)
			`)
			if err != nil {
				return err
			}

			// Create group_supervisor junction table
			_, err = db.ExecContext(ctx, `
				CREATE TABLE IF NOT EXISTS group_supervisors (
					id SERIAL PRIMARY KEY,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
					supervisor_id INTEGER NOT NULL REFERENCES pedagogical_specialists(id) ON DELETE CASCADE,
					UNIQUE(group_id, supervisor_id)
				)
			`)
			if err != nil {
				return err
			}

			// Create combined_group_group junction table
			_, err = db.ExecContext(ctx, `
				CREATE TABLE IF NOT EXISTS combined_group_groups (
					id SERIAL PRIMARY KEY,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					combined_group_id INTEGER NOT NULL REFERENCES combined_groups(id) ON DELETE CASCADE,
					group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
					UNIQUE(combined_group_id, group_id)
				)
			`)
			if err != nil {
				return err
			}

			// Create combined_group_specialist junction table
			_, err = db.ExecContext(ctx, `
				CREATE TABLE IF NOT EXISTS combined_group_specialists (
					id SERIAL PRIMARY KEY,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					combined_group_id INTEGER NOT NULL REFERENCES combined_groups(id) ON DELETE CASCADE,
					specialist_id INTEGER NOT NULL REFERENCES pedagogical_specialists(id) ON DELETE CASCADE,
					UNIQUE(combined_group_id, specialist_id)
				)
			`)
			if err != nil {
				return err
			}

			// Create room_occupancy_supervisor junction table
			_, err = db.ExecContext(ctx, `
				CREATE TABLE IF NOT EXISTS room_occupancy_supervisors (
					id SERIAL PRIMARY KEY,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					room_occupancy_id INTEGER NOT NULL REFERENCES room_occupancy(id) ON DELETE CASCADE,
					supervisor_id INTEGER NOT NULL REFERENCES pedagogical_specialists(id) ON DELETE CASCADE,
					UNIQUE(room_occupancy_id, supervisor_id)
				)
			`)
			if err != nil {
				return err
			}

			// Add missing constraints to existing tables
			_, err = db.ExecContext(ctx, `
				ALTER TABLE groups 
				ADD COLUMN IF NOT EXISTS room_id INTEGER REFERENCES rooms(id) ON DELETE SET NULL,
				ADD COLUMN IF NOT EXISTS representative_id INTEGER REFERENCES students(id) ON DELETE SET NULL
			`)

			return err
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 5: Dropping settings and junction tables...")

			// Remove added columns from existing tables
			_, err := db.ExecContext(ctx, `
				ALTER TABLE groups 
				DROP COLUMN IF EXISTS room_id,
				DROP COLUMN IF EXISTS representative_id
			`)
			if err != nil {
				return err
			}

			// Drop tables in reverse order to handle dependencies
			_, err = db.ExecContext(ctx, `DROP TABLE IF EXISTS room_occupancy_supervisors CASCADE`)
			if err != nil {
				return err
			}

			_, err = db.ExecContext(ctx, `DROP TABLE IF EXISTS combined_group_specialists CASCADE`)
			if err != nil {
				return err
			}

			_, err = db.ExecContext(ctx, `DROP TABLE IF EXISTS combined_group_groups CASCADE`)
			if err != nil {
				return err
			}

			_, err = db.ExecContext(ctx, `DROP TABLE IF EXISTS group_supervisors CASCADE`)
			if err != nil {
				return err
			}

			_, err = db.ExecContext(ctx, `DROP TABLE IF EXISTS combined_groups CASCADE`)
			if err != nil {
				return err
			}

			_, err = db.ExecContext(ctx, `DROP TABLE IF EXISTS devices CASCADE`)
			if err != nil {
				return err
			}

			_, err = db.ExecContext(ctx, `DROP TABLE IF EXISTS settings CASCADE`)
			return err
		},
	)
}
