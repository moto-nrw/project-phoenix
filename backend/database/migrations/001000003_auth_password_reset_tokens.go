package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	AuthPasswordResetTokensVersion     = "1.0.3"
	AuthPasswordResetTokensDescription = "Create auth.password_reset_tokens table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[AuthPasswordResetTokensVersion] = &Migration{
		Version:     AuthPasswordResetTokensVersion,
		Description: AuthPasswordResetTokensDescription,
		DependsOn:   []string{"1.0.1"}, // Depends on auth.accounts
	}

	// Migration 1.0.3: Create auth.password_reset_tokens table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createAuthPasswordResetTokensTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropAuthPasswordResetTokensTable(ctx, db)
		},
	)
}

// createAuthPasswordResetTokensTable creates the auth.password_reset_tokens table
func createAuthPasswordResetTokensTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.0.3: Creating auth.password_reset_tokens table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create password_reset_tokens table for password management
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS auth.password_reset_tokens (
			id BIGSERIAL PRIMARY KEY,
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

	// Create trigger for updating updated_at column
	_, err = tx.ExecContext(ctx, `
		-- Trigger for password_reset_tokens
		DROP TRIGGER IF EXISTS update_password_reset_tokens_updated_at ON auth.password_reset_tokens;
		CREATE TRIGGER update_password_reset_tokens_updated_at
		BEFORE UPDATE ON auth.password_reset_tokens
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger for password_reset_tokens: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// dropAuthPasswordResetTokensTable drops the auth.password_reset_tokens table
func dropAuthPasswordResetTokensTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.0.3: Removing auth.password_reset_tokens table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop trigger first
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_password_reset_tokens_updated_at ON auth.password_reset_tokens;
	`)
	if err != nil {
		return fmt.Errorf("error dropping trigger for password_reset_tokens table: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS auth.password_reset_tokens CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping auth.password_reset_tokens table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
