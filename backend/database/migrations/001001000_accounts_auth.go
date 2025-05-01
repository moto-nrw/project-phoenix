package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	AccountsAuthVersion     = "1.1.0"
	AccountsAuthDescription = "Authentication foundation tables"
)

func init() {
	// Migration 1.1.0: Authentication foundation tables
	Migrations.MustRegister(
		// Up function
		func(ctx context.Context, db *bun.DB) error {
			return accountsAuthUp(ctx, db)
		},
		// Down function
		func(ctx context.Context, db *bun.DB) error {
			return accountsAuthDown(ctx, db)
		},
	)
}

// accountsAuthUp creates the authentication foundation tables
func accountsAuthUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Creating authentication foundation tables...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Create the accounts table - the core login entity
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS accounts (
			id SERIAL PRIMARY KEY,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_login TIMESTAMP,
			email TEXT NOT NULL,
			username TEXT UNIQUE,
			name TEXT NOT NULL,
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
		CREATE INDEX IF NOT EXISTS idx_accounts_email ON accounts(email);
		CREATE INDEX IF NOT EXISTS idx_accounts_username ON accounts(username);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for accounts table: %w", err)
	}

	// 2. Create the tokens table - for auth tokens and session management
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS tokens (
			id SERIAL PRIMARY KEY,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			account_id INTEGER NOT NULL,
			token TEXT NOT NULL,
			expiry TIMESTAMP NOT NULL,
			mobile BOOLEAN NOT NULL DEFAULT FALSE,
			identifier TEXT,
			CONSTRAINT fk_tokens_account FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating tokens table: %w", err)
	}

	// Create indexes for tokens
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_tokens_account_id ON tokens(account_id);
		CREATE INDEX IF NOT EXISTS idx_tokens_token ON tokens(token);
		CREATE INDEX IF NOT EXISTS idx_tokens_expiry ON tokens(expiry);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for tokens table: %w", err)
	}

	// 3. Create password_reset_tokens table for password management
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS password_reset_tokens (
			id SERIAL PRIMARY KEY,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			account_id INTEGER NOT NULL,
			token TEXT NOT NULL,
			expiry TIMESTAMP NOT NULL,
			used BOOLEAN NOT NULL DEFAULT FALSE,
			CONSTRAINT fk_password_reset_tokens_account FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating password_reset_tokens table: %w", err)
	}

	// Create indexes for password_reset_tokens
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_account_id ON password_reset_tokens(account_id);
		CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_token ON password_reset_tokens(token);
		CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_expiry ON password_reset_tokens(expiry);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for password_reset_tokens table: %w", err)
	}

	// Create a trigger to update updated_at timestamp
	_, err = tx.ExecContext(ctx, `
		-- Function to update updated_at or modified_at timestamp
		CREATE OR REPLACE FUNCTION update_updated_at_column()
		RETURNS TRIGGER AS $$
		BEGIN
			-- Check which column exists in the table and update accordingly
			IF TG_TABLE_NAME = 'custom_users' OR TG_TABLE_NAME = 'pedagogical_specialist' OR 
			   TG_TABLE_NAME = 'rfid_cards' OR TG_TABLE_NAME = 'device' THEN
				NEW.modified_at = CURRENT_TIMESTAMP;
			ELSE
				NEW.updated_at = CURRENT_TIMESTAMP;
			END IF;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		
		-- Trigger for accounts
		DROP TRIGGER IF EXISTS update_accounts_updated_at ON accounts;
		CREATE TRIGGER update_accounts_updated_at
		BEFORE UPDATE ON accounts
		FOR EACH ROW
		EXECUTE FUNCTION update_updated_at_column();
		
		-- Trigger for tokens
		DROP TRIGGER IF EXISTS update_tokens_updated_at ON tokens;
		CREATE TRIGGER update_tokens_updated_at
		BEFORE UPDATE ON tokens
		FOR EACH ROW
		EXECUTE FUNCTION update_updated_at_column();
		
		-- Trigger for password_reset_tokens
		DROP TRIGGER IF EXISTS update_password_reset_tokens_updated_at ON password_reset_tokens;
		CREATE TRIGGER update_password_reset_tokens_updated_at
		BEFORE UPDATE ON password_reset_tokens
		FOR EACH ROW
		EXECUTE FUNCTION update_updated_at_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at triggers: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// accountsAuthDown removes the authentication foundation tables
func accountsAuthDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back authentication foundation tables...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop tables in reverse order of dependencies
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS password_reset_tokens;
		DROP TABLE IF EXISTS tokens;
		DROP TABLE IF EXISTS accounts;
		
		-- Drop the function if no other tables are using it
		DROP FUNCTION IF EXISTS update_updated_at_column();
	`)
	if err != nil {
		return fmt.Errorf("error dropping authentication tables: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
