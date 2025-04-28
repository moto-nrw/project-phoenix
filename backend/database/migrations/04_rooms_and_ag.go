package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	// Migration 4: Room and Activity Group tables
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 4: Creating room and activity group tables...")

			// First, check for and create student-related tables
			fmt.Println("Creating student tables first (if needed)...")

			// Create custom_users table if it doesn't exist
			var tableCount int
			err := db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'custom_users'").Scan(&tableCount)
			if err != nil {
				return fmt.Errorf("error checking if custom_users table exists: %w", err)
			}

			if tableCount == 0 {
				_, err = db.ExecContext(ctx, `
					CREATE TABLE IF NOT EXISTS custom_users (
						id SERIAL PRIMARY KEY,
						first_name TEXT NOT NULL,
						second_name TEXT NOT NULL,
						tag_id TEXT UNIQUE,
						account_id BIGINT UNIQUE,
						created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
						modified_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
					)
				`)
				if err != nil {
					return err
				}
			}

			// Create groups table if it doesn't exist
			err = db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'groups'").Scan(&tableCount)
			if err != nil {
				return fmt.Errorf("error checking if groups table exists: %w", err)
			}

			if tableCount == 0 {
				_, err = db.ExecContext(ctx, `
					CREATE TABLE IF NOT EXISTS groups (
						id SERIAL PRIMARY KEY,
						name TEXT NOT NULL,
						created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
						modified_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
					)
				`)
				if err != nil {
					return err
				}
			}

			// Create students table if it doesn't exist
			err = db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'students'").Scan(&tableCount)
			if err != nil {
				return fmt.Errorf("error checking if students table exists: %w", err)
			}

			if tableCount == 0 {
				_, err = db.ExecContext(ctx, `
					CREATE TABLE IF NOT EXISTS students (
						id SERIAL PRIMARY KEY,
						school_class TEXT NOT NULL,
						bus BOOLEAN NOT NULL DEFAULT FALSE,
						name_lg TEXT NOT NULL,
						contact_lg TEXT NOT NULL,
						in_house BOOLEAN NOT NULL DEFAULT FALSE,
						wc BOOLEAN NOT NULL DEFAULT FALSE,
						school_yard BOOLEAN NOT NULL DEFAULT FALSE,
						custom_user_id BIGINT NOT NULL,
						group_id BIGINT NOT NULL,
						created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
						modified_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
					)
				`)
				if err != nil {
					return err
				}
			}

			// Now create the rooms table
			_, err = db.ExecContext(ctx, `
				CREATE TABLE IF NOT EXISTS rooms (
					id SERIAL PRIMARY KEY,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					modified_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					room_name TEXT NOT NULL UNIQUE,
					building TEXT,
					floor INTEGER NOT NULL DEFAULT 0,
					capacity INTEGER NOT NULL,
					category TEXT NOT NULL DEFAULT 'Other',
					color TEXT NOT NULL DEFAULT '#FFFFFF'
				)
			`)
			if err != nil {
				return err
			}

			// Create ag_categories table
			_, err = db.ExecContext(ctx, `
				CREATE TABLE IF NOT EXISTS ag_categories (
					id SERIAL PRIMARY KEY,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					name TEXT NOT NULL UNIQUE
				)
			`)
			if err != nil {
				return err
			}

			// Create pedagogical_specialists table (referenced by ag)
			_, err = db.ExecContext(ctx, `
				CREATE TABLE IF NOT EXISTS pedagogical_specialists (
					id SERIAL PRIMARY KEY,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					modified_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
					name TEXT NOT NULL,
					position TEXT,
					is_teacher BOOLEAN NOT NULL DEFAULT FALSE
				)
			`)
			if err != nil {
				return err
			}

			// Create ags table
			_, err = db.ExecContext(ctx, `
				CREATE TABLE IF NOT EXISTS ags (
					id SERIAL PRIMARY KEY,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					modified_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					name TEXT NOT NULL,
					max_participant INTEGER NOT NULL,
					is_open_ag BOOLEAN NOT NULL DEFAULT FALSE,
					supervisor_id INTEGER NOT NULL REFERENCES pedagogical_specialists(id) ON DELETE RESTRICT,
					ag_category_id INTEGER NOT NULL REFERENCES ag_categories(id) ON DELETE RESTRICT,
					datespan_id INTEGER REFERENCES timespans(id) ON DELETE SET NULL
				)
			`)
			if err != nil {
				return err
			}

			// Create ag_times table
			_, err = db.ExecContext(ctx, `
				CREATE TABLE IF NOT EXISTS ag_times (
					id SERIAL PRIMARY KEY,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					weekday TEXT NOT NULL CHECK (weekday IN ('Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday')),
					timespan_id INTEGER NOT NULL REFERENCES timespans(id) ON DELETE CASCADE,
					ag_id INTEGER NOT NULL REFERENCES ags(id) ON DELETE CASCADE
				)
			`)
			if err != nil {
				return err
			}

			// Create student_ags junction table
			_, err = db.ExecContext(ctx, `
				CREATE TABLE IF NOT EXISTS student_ags (
					id SERIAL PRIMARY KEY,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					student_id INTEGER NOT NULL REFERENCES students(id) ON DELETE CASCADE,
					ag_id INTEGER NOT NULL REFERENCES ags(id) ON DELETE CASCADE,
					UNIQUE(student_id, ag_id)
				)
			`)
			if err != nil {
				return err
			}

			// Create room_occupancy table
			_, err = db.ExecContext(ctx, `
				CREATE TABLE IF NOT EXISTS room_occupancy (
					id SERIAL PRIMARY KEY,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					room_id INTEGER NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
					timespan_id INTEGER NOT NULL REFERENCES timespans(id) ON DELETE CASCADE,
					UNIQUE(room_id, timespan_id)
				)
			`)

			return err
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 4: Dropping room and activity group tables...")

			// Drop tables in reverse order to handle dependencies
			_, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS room_occupancy CASCADE`)
			if err != nil {
				return err
			}

			_, err = db.ExecContext(ctx, `DROP TABLE IF EXISTS student_ags CASCADE`)
			if err != nil {
				return err
			}

			_, err = db.ExecContext(ctx, `DROP TABLE IF EXISTS ag_times CASCADE`)
			if err != nil {
				return err
			}

			_, err = db.ExecContext(ctx, `DROP TABLE IF EXISTS ags CASCADE`)
			if err != nil {
				return err
			}

			_, err = db.ExecContext(ctx, `DROP TABLE IF EXISTS pedagogical_specialists CASCADE`)
			if err != nil {
				return err
			}

			_, err = db.ExecContext(ctx, `DROP TABLE IF EXISTS ag_categories CASCADE`)
			if err != nil {
				return err
			}

			_, err = db.ExecContext(ctx, `DROP TABLE IF EXISTS rooms CASCADE`)
			return err
		},
	)
}
