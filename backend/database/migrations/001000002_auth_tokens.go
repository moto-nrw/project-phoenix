package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	AuthTokensVersion     = "1.0.2"
	AuthTokensDescription = "Create auth.tokens table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[AuthTokensVersion] = &Migration{
		Version:     AuthTokensVersion,
		Description: AuthTokensDescription,
		DependsOn:   []string{"1.0.1"}, // Depends on auth.accounts
	}

	// Migration 1.0.2: Create auth.tokens table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createAuthTokensTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropAuthTokensTable(ctx, db)
		},
	)
}

// createAuthTokensTable creates the auth.tokens table
func createAuthTokensTable(ctx context.Context, db *bun.DB) error {
	LogMigration(AuthTokensVersion, "Creating auth.tokens table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			logRollbackError(err)
		}
	}()

	// Create the tokens table - for auth tokens and session management
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS auth.tokens (
			id BIGSERIAL PRIMARY KEY,
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

	// Create trigger for updating updated_at column
	_, err = tx.ExecContext(ctx, `
		-- Trigger for tokens
		DROP TRIGGER IF EXISTS update_tokens_updated_at ON auth.tokens;
		CREATE TRIGGER update_tokens_updated_at
		BEFORE UPDATE ON auth.tokens
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger for tokens: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// dropAuthTokensTable drops the auth.tokens table
func dropAuthTokensTable(ctx context.Context, db *bun.DB) error {
	LogMigration(AuthTokensVersion, "Rolling back: Removing auth.tokens table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			logRollbackError(err)
		}
	}()

	// Drop trigger first
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_tokens_updated_at ON auth.tokens;
	`)
	if err != nil {
		return fmt.Errorf("error dropping trigger for tokens table: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS auth.tokens CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping auth.tokens table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
