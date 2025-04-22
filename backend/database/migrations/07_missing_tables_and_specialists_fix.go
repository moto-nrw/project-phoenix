package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	// Migration 7: Add missing tables and fix PedagogicalSpecialist structure
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 7: Adding missing tables and fixing PedagogicalSpecialist structure...")

			// 1. Create Room_history table
			_, err := db.ExecContext(ctx, `
				CREATE TABLE IF NOT EXISTS room_history (
					id SERIAL PRIMARY KEY,
					room_id INTEGER NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
					ag_name TEXT NOT NULL,
					day DATE NOT NULL,
					timespan_id INTEGER NOT NULL REFERENCES timespans(id) ON DELETE CASCADE,
					ag_category_id INTEGER REFERENCES ag_categories(id) ON DELETE SET NULL,
					supervisor_id INTEGER NOT NULL,
					max_participant INTEGER NOT NULL DEFAULT 0,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
				)
			`)
			if err != nil {
				return err
			}

			// Drop constraints for tables that reference pedagogical_specialists
			_, err = db.ExecContext(ctx, `
				-- Drop existing junction tables for specialists to allow restructuring
				DROP TABLE IF EXISTS room_occupancy_supervisors CASCADE;
				DROP TABLE IF EXISTS group_supervisors CASCADE;
				DROP TABLE IF EXISTS combined_group_specialists CASCADE;
				
				-- Drop related dependent tables
				DROP TABLE IF EXISTS ag_times CASCADE;
				DROP TABLE IF EXISTS student_ags CASCADE;
				DROP TABLE IF EXISTS ags CASCADE;
				
				-- Drop and recreate pedagogical_specialists with proper structure
				DROP TABLE IF EXISTS pedagogical_specialists CASCADE;
				
				-- Create the new pedagogical_specialists table with proper fields
				CREATE TABLE pedagogical_specialists (
					id SERIAL PRIMARY KEY,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					modified_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
					name TEXT NOT NULL,
					position TEXT,
					is_teacher BOOLEAN NOT NULL DEFAULT FALSE
				);
				
				-- Recreate junction tables
				CREATE TABLE room_occupancy_supervisors (
					id SERIAL PRIMARY KEY,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					room_occupancy_id INTEGER NOT NULL REFERENCES room_occupancy(id) ON DELETE CASCADE,
					supervisor_id INTEGER NOT NULL REFERENCES pedagogical_specialists(id) ON DELETE CASCADE,
					UNIQUE(room_occupancy_id, supervisor_id)
				);
				
				CREATE TABLE group_supervisors (
					id SERIAL PRIMARY KEY,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
					supervisor_id INTEGER NOT NULL REFERENCES pedagogical_specialists(id) ON DELETE CASCADE,
					UNIQUE(group_id, supervisor_id)
				);
				
				CREATE TABLE combined_group_specialists (
					id SERIAL PRIMARY KEY,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					combined_group_id INTEGER NOT NULL REFERENCES combined_groups(id) ON DELETE CASCADE,
					specialist_id INTEGER NOT NULL REFERENCES pedagogical_specialists(id) ON DELETE CASCADE,
					UNIQUE(combined_group_id, specialist_id)
				);
				
				-- Recreate ag_categories table if needed
				CREATE TABLE IF NOT EXISTS ag_categories (
					id SERIAL PRIMARY KEY,
					name TEXT NOT NULL UNIQUE,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
				);
				
				-- Create ags table with proper references
				CREATE TABLE ags (
					id SERIAL PRIMARY KEY,
					name TEXT NOT NULL,
					max_participant INTEGER NOT NULL,
					is_open_ag BOOLEAN NOT NULL DEFAULT FALSE,
					supervisor_id INTEGER NOT NULL REFERENCES pedagogical_specialists(id) ON DELETE RESTRICT,
					ag_category_id INTEGER NOT NULL REFERENCES ag_categories(id) ON DELETE RESTRICT,
					datespan_id INTEGER REFERENCES timespans(id) ON DELETE SET NULL,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					modified_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
				);
				
				-- Create ag_times table
				CREATE TABLE ag_times (
					id SERIAL PRIMARY KEY,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					weekday TEXT NOT NULL CHECK (weekday IN ('Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday')),
					timespan_id INTEGER NOT NULL REFERENCES timespans(id) ON DELETE CASCADE,
					ag_id INTEGER NOT NULL REFERENCES ags(id) ON DELETE CASCADE
				);
				
				-- Create student_ags junction table
				CREATE TABLE student_ags (
					id SERIAL PRIMARY KEY,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					student_id INTEGER NOT NULL REFERENCES students(id) ON DELETE CASCADE,
					ag_id INTEGER NOT NULL REFERENCES ags(id) ON DELETE CASCADE,
					UNIQUE(student_id, ag_id)
				);
			`)
			if err != nil {
				return err
			}

			// Create indexes on room_history
			_, err = db.ExecContext(ctx, `
				CREATE INDEX IF NOT EXISTS idx_room_history_day ON room_history(day);
				CREATE INDEX IF NOT EXISTS idx_room_history_room_id ON room_history(room_id);
			`)

			return err
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 7: Dropping room_history and reverting PedagogicalSpecialist structure...")

			// 1. Drop room_history table and its indexes
			_, err := db.ExecContext(ctx, `
				DROP INDEX IF EXISTS idx_room_history_day;
				DROP INDEX IF EXISTS idx_room_history_room_id;
				DROP TABLE IF EXISTS room_history CASCADE;
			`)
			if err != nil {
				return err
			}

			// 2. Drop all related tables and recreate the original structure
			_, err = db.ExecContext(ctx, `
				-- Drop all tables created/modified in this migration
				DROP TABLE IF EXISTS student_ags CASCADE;
				DROP TABLE IF EXISTS ag_times CASCADE;
				DROP TABLE IF EXISTS ags CASCADE;
				DROP TABLE IF EXISTS ag_categories CASCADE;
				DROP TABLE IF EXISTS combined_group_specialists CASCADE;
				DROP TABLE IF EXISTS group_supervisors CASCADE;
				DROP TABLE IF EXISTS room_occupancy_supervisors CASCADE;
				DROP TABLE IF EXISTS pedagogical_specialists CASCADE;
				
				-- Recreate the original pedagogical_specialists table
				CREATE TABLE pedagogical_specialists (
					id SERIAL PRIMARY KEY,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					modified_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
					name TEXT NOT NULL,
					position TEXT,
					is_teacher BOOLEAN NOT NULL DEFAULT FALSE
				);
			`)

			return err
		},
	)
}
