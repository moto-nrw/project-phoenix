package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	AuthAccountsVersion     = "1.0.1"
	AuthAccountsDescription = "Create auth.accounts table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[AuthAccountsVersion] = &Migration{
		Version:     AuthAccountsVersion,
		Description: AuthAccountsDescription,
		DependsOn:   []string{"0.0.0", "0.1.0", "1.0.0"}, // Depends on schemas, core functions and infrastructure
	}

	// Migration 1.0.1: Create auth.accounts table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createAuthAccountsTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropAuthAccountsTable(ctx, db)
		},
	)
}

// createAuthAccountsTable creates the auth.accounts table
func createAuthAccountsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.0.1: Creating auth.accounts table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create auth function for updating timestamps if it doesn't exist
	_, err = tx.ExecContext(ctx, `
		-- Create or replace the function for updating timestamps
		CREATE OR REPLACE FUNCTION auth.update_updated_at_column()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.updated_at = CURRENT_TIMESTAMP;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
	`)
	if err != nil {
		return fmt.Errorf("error creating auth timestamp function: %w", err)
	}

	// Create the accounts table - the core login entity
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS auth.accounts (
			id BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_login TIMESTAMPTZ,
			email TEXT NOT NULL,
			username TEXT UNIQUE,
			active BOOLEAN NOT NULL DEFAULT TRUE,
			roles TEXT[],
			password_hash TEXT
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating accounts table: %w", err)
	}

	// Create indexes for accounts
	_, err = tx.ExecContext(ctx, `
		CREATE UNIQUE INDEX IF NOT EXISTS idx_accounts_email ON auth.accounts(email);
		CREATE INDEX IF NOT EXISTS idx_accounts_username ON auth.accounts(username);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for accounts table: %w", err)
	}

	// Create trigger for updating updated_at column
	_, err = tx.ExecContext(ctx, `
		-- Trigger for accounts
		DROP TRIGGER IF EXISTS update_accounts_updated_at ON auth.accounts;
		CREATE TRIGGER update_accounts_updated_at
		BEFORE UPDATE ON auth.accounts
		FOR EACH ROW
		EXECUTE FUNCTION auth.update_updated_at_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger for accounts: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// dropAuthAccountsTable drops the auth.accounts table
func dropAuthAccountsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.0.1: Removing auth.accounts table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop trigger first
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_accounts_updated_at ON auth.accounts;
	`)
	if err != nil {
		return fmt.Errorf("error dropping trigger for accounts table: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS auth.accounts CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping auth.accounts table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
