package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	UserFoundationVersion     = "1.3.0"
	UserFoundationDescription = "User foundation tables"
)

func init() {
	// Register the migration
	migration := &Migration{
		Version:     UserFoundationVersion,
		Description: UserFoundationDescription,
		DependsOn:   []string{"1.2.0"}, // Depends on foundation tables
		Up:          userFoundationUp,
		Down:        userFoundationDown,
	}

	registerMigration(migration)
}

// userFoundationUp creates the user foundation tables
func userFoundationUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Creating user foundation tables...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Create the custom_user table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS custom_user (
			id BIGSERIAL PRIMARY KEY,
			first_name TEXT NOT NULL,
			second_name TEXT NOT NULL,
			tag_id TEXT UNIQUE,
			account_id INTEGER UNIQUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			modified_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT fk_custom_user_account FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE SET NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating custom_user table: %w", err)
	}

	// Create indexes for custom_user
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_custom_user_account_id ON custom_user(account_id);
		CREATE INDEX IF NOT EXISTS idx_custom_user_tag_id ON custom_user(tag_id);
		CREATE INDEX IF NOT EXISTS idx_custom_user_names ON custom_user(first_name, second_name);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for custom_user table: %w", err)
	}

	// 2. Create the profile table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS profile (
			id BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			account_id INTEGER NOT NULL UNIQUE,
			avatar TEXT,
			bio TEXT,
			settings JSONB DEFAULT '{}'::jsonb,
			CONSTRAINT fk_profile_account FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating profile table: %w", err)
	}

	// Create indexes for profile
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_profile_account_id ON profile(account_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for profile table: %w", err)
	}

	// Create triggers for updated_at columns
	_, err = tx.ExecContext(ctx, `
		-- Function to update updated_at column already created in previous migration
		
		-- Trigger for custom_user
		DROP TRIGGER IF EXISTS update_custom_user_modified_at ON custom_user;
		CREATE TRIGGER update_custom_user_modified_at
		BEFORE UPDATE ON custom_user
		FOR EACH ROW
		EXECUTE FUNCTION update_updated_at_column();
		
		-- Trigger for profile
		DROP TRIGGER IF EXISTS update_profile_updated_at ON profile;
		CREATE TRIGGER update_profile_updated_at
		BEFORE UPDATE ON profile
		FOR EACH ROW
		EXECUTE FUNCTION update_updated_at_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at triggers: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// userFoundationDown removes the user foundation tables
func userFoundationDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back user foundation tables...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop tables in reverse order of dependencies
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS profile;
		DROP TABLE IF EXISTS custom_user;
	`)
	if err != nil {
		return fmt.Errorf("error dropping user foundation tables: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
