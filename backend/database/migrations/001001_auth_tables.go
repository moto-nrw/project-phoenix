package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	AuthTablesVersion     = "1.1.0"
	AuthTablesDescription = "Authentication foundation tables"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[AuthTablesVersion] = &Migration{
		Version:     AuthTablesVersion,
		Description: AuthTablesDescription,
		DependsOn:   []string{"0.0.0", "0.1.0", "1.0.0"}, // Depends on schemas, core functions and infrastructure
	}

	// Migration 1.1.0: Authentication foundation tables
	Migrations.MustRegister(
		// Up function
		func(ctx context.Context, db *bun.DB) error {
			return authTablesUp(ctx, db)
		},
		// Down function
		func(ctx context.Context, db *bun.DB) error {
			return authTablesDown(ctx, db)
		},
	)
}

// authTablesUp creates the authentication foundation tables
func authTablesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.1.0: Creating authentication foundation tables...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Create the accounts table - the core login entity
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS auth.accounts (
			id SERIAL PRIMARY KEY,
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

	// 2. Create the tokens table - for auth tokens and session management
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS auth.tokens (
			id SERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			account_id BIGINT NOT NULL,
			token TEXT NOT NULL,
			expiry TIMESTAMPTZ NOT NULL,
			mobile BOOLEAN NOT NULL DEFAULT FALSE,
			identifier TEXT,
			CONSTRAINT fk_tokens_account FOREIGN KEY (account_id) REFERENCES auth.accounts(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating tokens table: %w", err)
	}

	// Create indexes for tokens
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_tokens_account_id ON auth.tokens(account_id);
		CREATE INDEX IF NOT EXISTS idx_tokens_token ON auth.tokens(token);
		CREATE INDEX IF NOT EXISTS idx_tokens_expiry ON auth.tokens(expiry);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for tokens table: %w", err)
	}

	// 3. Create password_reset_tokens table for password management
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS auth.password_reset_tokens (
			id SERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			account_id BIGINT NOT NULL,
			token TEXT NOT NULL,
			expiry TIMESTAMPTZ NOT NULL,
			used BOOLEAN NOT NULL DEFAULT FALSE,
			CONSTRAINT fk_password_reset_tokens_account FOREIGN KEY (account_id) REFERENCES auth.accounts(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating password_reset_tokens table: %w", err)
	}

	// Create indexes for password_reset_tokens
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_account_id ON auth.password_reset_tokens(account_id);
		CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_token ON auth.password_reset_tokens(token);
		CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_expiry ON auth.password_reset_tokens(expiry);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for password_reset_tokens table: %w", err)
	}

	// Create or update the trigger for updated_at timestamps
	_, err = tx.ExecContext(ctx, `
		-- Create or replace the function for updating timestamps
		CREATE OR REPLACE FUNCTION auth.update_updated_at_column()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.updated_at = CURRENT_TIMESTAMP;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		
		-- Trigger for accounts
		DROP TRIGGER IF EXISTS update_accounts_updated_at ON auth.accounts;
		CREATE TRIGGER update_accounts_updated_at
		BEFORE UPDATE ON auth.accounts
		FOR EACH ROW
		EXECUTE FUNCTION auth.update_updated_at_column();
		
		-- Trigger for tokens
		DROP TRIGGER IF EXISTS update_tokens_updated_at ON auth.tokens;
		CREATE TRIGGER update_tokens_updated_at
		BEFORE UPDATE ON auth.tokens
		FOR EACH ROW
		EXECUTE FUNCTION auth.update_updated_at_column();
		
		-- Trigger for password_reset_tokens
		DROP TRIGGER IF EXISTS update_password_reset_tokens_updated_at ON auth.password_reset_tokens;
		CREATE TRIGGER update_password_reset_tokens_updated_at
		BEFORE UPDATE ON auth.password_reset_tokens
		FOR EACH ROW
		EXECUTE FUNCTION auth.update_updated_at_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at triggers: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// authTablesDown removes the authentication foundation tables
func authTablesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.1.0: Removing authentication foundation tables...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop tables in reverse order of dependencies
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS auth.password_reset_tokens;
		DROP TABLE IF EXISTS auth.tokens;
		DROP TABLE IF EXISTS auth.accounts;
		
		-- Drop the function
		DROP FUNCTION IF EXISTS auth.update_updated_at_column();
	`)
	if err != nil {
		return fmt.Errorf("error dropping authentication tables: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
