package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/uptrace/bun"
)

const (
	UsersProfilesVersion     = "1.2.2"
	UsersProfilesDescription = "Users profiles table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[UsersProfilesVersion] = &Migration{
		Version:     UsersProfilesVersion,
		Description: UsersProfilesDescription,
		DependsOn:   []string{"1.0.9"}, // Depends on auth.accounts (FK), not users.persons
	}

	// Migration 1.2.2: Users profiles table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return usersProfilesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return usersProfilesDown(ctx, db)
		},
	)
}

// usersProfilesUp creates the users.profiles table
func usersProfilesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.2.2: Creating users.profiles table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logger.Logger.Warnf("Error rolling back transaction: %v", err)
		}
	}()

	// Create the profiles table - user profile information
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.profiles (
			id BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			account_id BIGINT NOT NULL UNIQUE,
			avatar TEXT,
			bio TEXT,
			settings JSONB DEFAULT '{}'::jsonb,
			CONSTRAINT fk_profiles_account FOREIGN KEY (account_id) REFERENCES auth.accounts(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating profiles table: %w", err)
	}

	// Create indexes for profiles
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_profiles_account_id ON users.profiles(account_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for profiles table: %w", err)
	}

	// Create updated_at timestamp trigger
	_, err = tx.ExecContext(ctx, `
		-- Trigger for profiles
		DROP TRIGGER IF EXISTS update_profiles_updated_at ON users.profiles;
		CREATE TRIGGER update_profiles_updated_at
		BEFORE UPDATE ON users.profiles
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// usersProfilesDown removes the users.profiles table
func usersProfilesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.2.2: Removing users.profiles table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logger.Logger.Warnf("Error rolling back transaction: %v", err)
		}
	}()

	// Drop the profiles table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS users.profiles;
	`)
	if err != nil {
		return fmt.Errorf("error dropping users.profiles table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
