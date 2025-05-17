package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	AuthAccountRolesVersion     = "1.0.7"
	AuthAccountRolesDescription = "Create auth.account_roles table for account-role assignments"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[AuthAccountRolesVersion] = &Migration{
		Version:     AuthAccountRolesVersion,
		Description: AuthAccountRolesDescription,
		DependsOn:   []string{"1.0.1", "1.0.4"}, // Depends on auth.accounts and auth.roles
	}

	// Migration 1.0.7: Create auth.account_roles table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createAuthAccountRolesTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropAuthAccountRolesTable(ctx, db)
		},
	)
}

// createAuthAccountRolesTable creates the auth.account_roles table
func createAuthAccountRolesTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.0.7: Creating auth.account_roles table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create the account_roles table - for mapping accounts to roles
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS auth.account_roles (
			id BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			account_id BIGINT NOT NULL,
			role_id BIGINT NOT NULL,
			
			CONSTRAINT fk_account_roles_account 
				FOREIGN KEY (account_id) 
				REFERENCES auth.accounts(id) 
				ON DELETE CASCADE,
				
			CONSTRAINT fk_account_roles_role 
				FOREIGN KEY (role_id) 
				REFERENCES auth.roles(id) 
				ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating account_roles table: %w", err)
	}

	// Create indexes for account_roles
	_, err = tx.ExecContext(ctx, `
		-- Unique constraint to prevent duplicate account-role assignments
		CREATE UNIQUE INDEX IF NOT EXISTS idx_account_roles_account_role 
		ON auth.account_roles(account_id, role_id);
		
		-- Index for efficient account lookups
		CREATE INDEX IF NOT EXISTS idx_account_roles_account_id 
		ON auth.account_roles(account_id);
		
		-- Index for efficient role lookups
		CREATE INDEX IF NOT EXISTS idx_account_roles_role_id 
		ON auth.account_roles(role_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for account_roles table: %w", err)
	}

	// Create trigger for updating updated_at column
	_, err = tx.ExecContext(ctx, `
		-- Trigger for account_roles
		DROP TRIGGER IF EXISTS update_account_roles_updated_at ON auth.account_roles;
		CREATE TRIGGER update_account_roles_updated_at
		BEFORE UPDATE ON auth.account_roles
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger for account_roles: %w", err)
	}

	// Note: The original migration tried to update a non-existent 'roles' column in auth.accounts
	// We'll skip creating those triggers since the column doesn't exist
	// The account_roles table itself is sufficient for managing role assignments

	// Commit the transaction
	return tx.Commit()
}

// dropAuthAccountRolesTable drops the auth.account_roles table
func dropAuthAccountRolesTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.0.7: Removing auth.account_roles table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop triggers first
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_account_roles_updated_at ON auth.account_roles;
	`)
	if err != nil {
		return fmt.Errorf("error dropping triggers for account_roles table: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS auth.account_roles CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping auth.account_roles table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
