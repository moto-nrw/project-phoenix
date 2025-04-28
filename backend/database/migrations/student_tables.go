package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// These functions provide helpers for ensuring student-related tables are created
// They're meant to be used by other migrations that depend on students tables

// ensureStudentTablesExist creates the necessary tables for student functionality if they don't exist
func ensureStudentTablesExist(ctx context.Context, db *bun.DB) error {
	// First check if the custom_users table exists
	var tableCount int
	err := db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'custom_users'").Scan(&tableCount)
	if err != nil {
		return fmt.Errorf("error checking if custom_users table exists: %w", err)
	}

	if tableCount == 0 {
		fmt.Println("Creating custom_users table...")
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
			return fmt.Errorf("error creating custom_users table: %w", err)
		}
	}

	// Check if the groups table exists
	err = db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'groups'").Scan(&tableCount)
	if err != nil {
		return fmt.Errorf("error checking if groups table exists: %w", err)
	}

	if tableCount == 0 {
		fmt.Println("Creating groups table...")
		_, err = db.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS groups (
				id SERIAL PRIMARY KEY,
				name TEXT NOT NULL,
				created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
				modified_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
			)
		`)
		if err != nil {
			return fmt.Errorf("error creating groups table: %w", err)
		}
	}

	// Check if the students table exists
	err = db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'students'").Scan(&tableCount)
	if err != nil {
		return fmt.Errorf("error checking if students table exists: %w", err)
	}

	if tableCount == 0 {
		fmt.Println("Creating students table...")
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
			return fmt.Errorf("error creating students table: %w", err)
		}
	}

	return nil
}
