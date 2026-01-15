package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/uptrace/bun"
)

const (
	UsersGuardianProfilesVersion     = "1.3.5.1"
	UsersGuardianProfilesDescription = "Create users.guardian_profiles table for storing guardian information"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[UsersGuardianProfilesVersion] = &Migration{
		Version:     UsersGuardianProfilesVersion,
		Description: UsersGuardianProfilesDescription,
		DependsOn:   []string{"1.0.9"}, // Depends on auth.accounts_parents
	}

	// Migration 1.3.5.1: Create guardian_profiles table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return usersGuardianProfilesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return usersGuardianProfilesDown(ctx, db)
		},
	)
}

// usersGuardianProfilesUp creates the users.guardian_profiles table
func usersGuardianProfilesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.3.5.1: Creating users.guardian_profiles table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logrus.Warnf("Error rolling back transaction: %v", err)
		}
	}()

	// Create the guardian_profiles table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.guardian_profiles (
			id BIGSERIAL PRIMARY KEY,

			-- Personal Information (REQUIRED)
			first_name TEXT NOT NULL,
			last_name TEXT NOT NULL,

			-- Contact Information (At least ONE required via constraint)
			email TEXT UNIQUE,
			phone TEXT,
			mobile_phone TEXT,

			-- Address (Optional)
			address_street TEXT,
			address_city TEXT,
			address_postal_code TEXT,

			-- Account Link (NULL if guardian doesn't have portal account)
			account_id BIGINT UNIQUE,
			has_account BOOLEAN NOT NULL DEFAULT FALSE,

			-- Preferences
			preferred_contact_method TEXT DEFAULT 'phone', -- email|phone|mobile|sms
			language_preference TEXT DEFAULT 'de',

			-- Additional Info
			occupation TEXT,
			employer TEXT,
			notes TEXT, -- Staff/admin notes about this guardian

			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

			-- Foreign Key
			CONSTRAINT fk_guardian_profile_account
				FOREIGN KEY (account_id) REFERENCES auth.accounts_parents(id) ON DELETE SET NULL,

			-- At least one contact method must exist
			CONSTRAINT check_contact_method CHECK (
				email IS NOT NULL OR
				phone IS NOT NULL OR
				mobile_phone IS NOT NULL
			)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating guardian_profiles table: %w", err)
	}

	// Create indexes for guardian_profiles
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_guardian_profiles_email ON users.guardian_profiles(email);
		CREATE INDEX IF NOT EXISTS idx_guardian_profiles_last_name ON users.guardian_profiles(last_name);
		CREATE INDEX IF NOT EXISTS idx_guardian_profiles_account_id ON users.guardian_profiles(account_id);
		CREATE INDEX IF NOT EXISTS idx_guardian_profiles_has_account ON users.guardian_profiles(has_account);
		CREATE INDEX IF NOT EXISTS idx_guardian_profiles_first_name ON users.guardian_profiles(first_name);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for guardian_profiles table: %w", err)
	}

	// Create updated_at timestamp trigger
	_, err = tx.ExecContext(ctx, `
		-- Trigger for guardian_profiles
		DROP TRIGGER IF EXISTS update_guardian_profiles_updated_at ON users.guardian_profiles;
		CREATE TRIGGER update_guardian_profiles_updated_at
		BEFORE UPDATE ON users.guardian_profiles
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// usersGuardianProfilesDown removes the users.guardian_profiles table
func usersGuardianProfilesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.3.5.1: Removing users.guardian_profiles table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logrus.Warnf("Error rolling back transaction: %v", err)
		}
	}()

	// Drop the trigger first
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_guardian_profiles_updated_at ON users.guardian_profiles;
	`)
	if err != nil {
		return fmt.Errorf("error dropping trigger: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS users.guardian_profiles CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping users.guardian_profiles table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
