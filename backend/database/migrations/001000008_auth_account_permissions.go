package migrations

import (
	"log"
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	AuthAccountPermissionsVersion     = "1.0.8"
	AuthAccountPermissionsDescription = "Create auth.account_permissions table for direct permission assignments"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[AuthAccountPermissionsVersion] = &Migration{
		Version:     AuthAccountPermissionsVersion,
		Description: AuthAccountPermissionsDescription,
		DependsOn:   []string{"1.0.1", "1.0.5"}, // Depends on auth.accounts and auth.permissions
	}

	// Migration 1.0.8: Create auth.account_permissions table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createAuthAccountPermissionsTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropAuthAccountPermissionsTable(ctx, db)
		},
	)
}

// createAuthAccountPermissionsTable creates the auth.account_permissions table
func createAuthAccountPermissionsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.0.8: Creating auth.account_permissions table...")

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

	// Create the account_permissions table - for direct permission assignments
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS auth.account_permissions (
			id BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			account_id BIGINT NOT NULL,
			permission_id BIGINT NOT NULL,
			granted BOOLEAN NOT NULL DEFAULT TRUE, -- If false, explicitly denies this permission
			
			CONSTRAINT fk_account_permissions_account 
				FOREIGN KEY (account_id) 
				REFERENCES auth.accounts(id) 
				ON DELETE CASCADE,
				
			CONSTRAINT fk_account_permissions_permission 
				FOREIGN KEY (permission_id) 
				REFERENCES auth.permissions(id) 
				ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating account_permissions table: %w", err)
	}

	// Create indexes for account_permissions
	_, err = tx.ExecContext(ctx, `
		-- Unique constraint to prevent duplicate account-permission assignments
		CREATE UNIQUE INDEX IF NOT EXISTS idx_account_permissions_account_permission 
		ON auth.account_permissions(account_id, permission_id);
		
		-- Index for efficient account lookups
		CREATE INDEX IF NOT EXISTS idx_account_permissions_account_id 
		ON auth.account_permissions(account_id);
		
		-- Index for efficient permission lookups
		CREATE INDEX IF NOT EXISTS idx_account_permissions_permission_id 
		ON auth.account_permissions(permission_id);
		
		-- Index for granted/denied lookups
		CREATE INDEX IF NOT EXISTS idx_account_permissions_granted 
		ON auth.account_permissions(granted);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for account_permissions table: %w", err)
	}

	// Create trigger for updating updated_at column
	_, err = tx.ExecContext(ctx, `
		-- Trigger for account_permissions
		DROP TRIGGER IF EXISTS update_account_permissions_updated_at ON auth.account_permissions;
		CREATE TRIGGER update_account_permissions_updated_at
		BEFORE UPDATE ON auth.account_permissions
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger for account_permissions: %w", err)
	}

	// Create function for permission checking
	_, err = tx.ExecContext(ctx, `
		-- Create a function to check if an account has a specific permission
		CREATE OR REPLACE FUNCTION auth.has_permission(account_id BIGINT, permission_name TEXT)
		RETURNS BOOLEAN AS $$
		DECLARE
			permission_granted BOOLEAN;
		BEGIN
			-- First check for direct denials (these override everything else)
			SELECT EXISTS (
				SELECT 1
				FROM auth.account_permissions ap
				JOIN auth.permissions p ON ap.permission_id = p.id
				WHERE ap.account_id = $1
				AND p.name = $2
				AND ap.granted = FALSE
			) INTO permission_granted;
			
			IF permission_granted THEN
				RETURN FALSE;
			END IF;
			
			-- Then check for direct grants
			SELECT EXISTS (
				SELECT 1
				FROM auth.account_permissions ap
				JOIN auth.permissions p ON ap.permission_id = p.id
				WHERE ap.account_id = $1
				AND p.name = $2
				AND ap.granted = TRUE
			) INTO permission_granted;
			
			IF permission_granted THEN
				RETURN TRUE;
			END IF;
			
			-- Finally check role-based permissions
			SELECT EXISTS (
				SELECT 1
				FROM auth.account_roles ar
				JOIN auth.role_permissions rp ON ar.role_id = rp.role_id
				JOIN auth.permissions p ON rp.permission_id = p.id
				WHERE ar.account_id = $1
				AND p.name = $2
			) INTO permission_granted;
			
			RETURN permission_granted;
		END;
		$$ LANGUAGE plpgsql;
		
		-- Create a function to check if an account has access to a resource
		CREATE OR REPLACE FUNCTION auth.has_resource_permission(account_id BIGINT, resource TEXT, action TEXT)
		RETURNS BOOLEAN AS $$
		DECLARE
			permission_granted BOOLEAN;
		BEGIN
			-- First check for direct denials (these override everything else)
			SELECT EXISTS (
				SELECT 1
				FROM auth.account_permissions ap
				JOIN auth.permissions p ON ap.permission_id = p.id
				WHERE ap.account_id = $1
				AND p.resource = $2
				AND p.action = $3
				AND ap.granted = FALSE
			) INTO permission_granted;
			
			IF permission_granted THEN
				RETURN FALSE;
			END IF;
			
			-- Then check for direct grants
			SELECT EXISTS (
				SELECT 1
				FROM auth.account_permissions ap
				JOIN auth.permissions p ON ap.permission_id = p.id
				WHERE ap.account_id = $1
				AND p.resource = $2
				AND p.action = $3
				AND ap.granted = TRUE
			) INTO permission_granted;
			
			IF permission_granted THEN
				RETURN TRUE;
			END IF;
			
			-- Finally check role-based permissions
			SELECT EXISTS (
				SELECT 1
				FROM auth.account_roles ar
				JOIN auth.role_permissions rp ON ar.role_id = rp.role_id
				JOIN auth.permissions p ON rp.permission_id = p.id
				WHERE ar.account_id = $1
				AND p.resource = $2
				AND p.action = $3
			) INTO permission_granted;
			
			RETURN permission_granted;
		END;
		$$ LANGUAGE plpgsql;
	`)
	if err != nil {
		return fmt.Errorf("error creating permission check functions: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// dropAuthAccountPermissionsTable drops the auth.account_permissions table
func dropAuthAccountPermissionsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.0.8: Removing auth.account_permissions table...")

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

	// Drop the permission check functions
	_, err = tx.ExecContext(ctx, `
		DROP FUNCTION IF EXISTS auth.has_permission(BIGINT, TEXT);
		DROP FUNCTION IF EXISTS auth.has_resource_permission(BIGINT, TEXT, TEXT);
	`)
	if err != nil {
		return fmt.Errorf("error dropping permission check functions: %w", err)
	}

	// Drop trigger first
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_account_permissions_updated_at ON auth.account_permissions;
	`)
	if err != nil {
		return fmt.Errorf("error dropping trigger for account_permissions table: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS auth.account_permissions CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping auth.account_permissions table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
