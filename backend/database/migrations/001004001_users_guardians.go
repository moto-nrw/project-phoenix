package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	UsersGuardiansVersion     = "1.4.1"
	UsersGuardiansDescription = "Create users.guardians table for guardian profiles"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[UsersGuardiansVersion] = &Migration{
		Version:     UsersGuardiansVersion,
		Description: UsersGuardiansDescription,
		DependsOn:   []string{"1.3.6"}, // Depends on students_guardians table
	}

	// Migration 1.4.1: Create users.guardians table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return usersGuardiansUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return usersGuardiansDown(ctx, db)
		},
	)
}

// usersGuardiansUp creates the users.guardians table
func usersGuardiansUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.4.1: Creating users.guardians table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Create the guardians table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.guardians (
			id BIGSERIAL PRIMARY KEY,
			account_id BIGINT UNIQUE REFERENCES auth.accounts_parents(id) ON DELETE SET NULL,
			first_name TEXT NOT NULL,
			last_name TEXT NOT NULL,
			phone TEXT NOT NULL,
			phone_secondary TEXT,
			email TEXT NOT NULL,
			address TEXT,
			city TEXT,
			postal_code TEXT,
			country TEXT NOT NULL DEFAULT 'DE',
			is_emergency_contact BOOLEAN NOT NULL DEFAULT FALSE,
			emergency_priority INT,
			notes TEXT,
			language_preference TEXT NOT NULL DEFAULT 'de',
			notification_preferences JSONB DEFAULT '{}',
			active BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_email_format CHECK (email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$')
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating guardians table: %w", err)
	}

	// Create indexes for guardians
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_guardians_account_id ON users.guardians(account_id);
		CREATE INDEX IF NOT EXISTS idx_guardians_email ON users.guardians(email);
		CREATE INDEX IF NOT EXISTS idx_guardians_phone ON users.guardians(phone);
		CREATE INDEX IF NOT EXISTS idx_guardians_last_name ON users.guardians(last_name);
		CREATE INDEX IF NOT EXISTS idx_guardians_active ON users.guardians(active);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for guardians table: %w", err)
	}

	// Create updated_at timestamp trigger
	_, err = tx.ExecContext(ctx, `
		-- Trigger for guardians
		DROP TRIGGER IF EXISTS update_guardians_updated_at ON users.guardians;
		CREATE TRIGGER update_guardians_updated_at
		BEFORE UPDATE ON users.guardians
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// usersGuardiansDown removes the users.guardians table
func usersGuardiansDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.4.1: Removing users.guardians table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Drop the trigger first
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_guardians_updated_at ON users.guardians;
	`)
	if err != nil {
		return fmt.Errorf("error dropping trigger for guardians table: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS users.guardians CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping users.guardians table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
