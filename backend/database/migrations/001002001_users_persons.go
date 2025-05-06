package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	UsersPersonsVersion     = "1.2.1"
	UsersPersonsDescription = "Users persons table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[UsersPersonsVersion] = &Migration{
		Version:     UsersPersonsVersion,
		Description: UsersPersonsDescription,
		DependsOn:   []string{"1.1.0"}, // Depends on auth tables
	}

	// Migration 1.2.1: Users persons table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return usersPersonsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return usersPersonsDown(ctx, db)
		},
	)
}

// usersPersonsUp creates the users.persons table
func usersPersonsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.2.1: Creating users.persons table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create the persons table - base entity for all individuals
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.persons (
			id BIGSERIAL PRIMARY KEY,
			first_name TEXT NOT NULL,
			last_name TEXT NOT NULL,
			tag_id TEXT UNIQUE,
			account_id BIGINT UNIQUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_persons_account FOREIGN KEY (account_id) REFERENCES auth.accounts(id) ON DELETE SET NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating persons table: %w", err)
	}

	// Create indexes for persons
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_persons_tag_id ON users.persons(tag_id);
		CREATE INDEX IF NOT EXISTS idx_persons_names ON users.persons(first_name, last_name);
		CREATE INDEX IF NOT EXISTS idx_persons_account_id ON users.persons(account_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for persons table: %w", err)
	}

	// Create updated_at timestamp trigger
	_, err = tx.ExecContext(ctx, `
		-- Trigger for persons
		DROP TRIGGER IF EXISTS update_persons_updated_at ON users.persons;
		CREATE TRIGGER update_persons_updated_at
		BEFORE UPDATE ON users.persons
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// usersPersonsDown removes the users.persons table
func usersPersonsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.2.1: Removing users.persons table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop the persons table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS users.persons;
	`)
	if err != nil {
		return fmt.Errorf("error dropping users.persons table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
