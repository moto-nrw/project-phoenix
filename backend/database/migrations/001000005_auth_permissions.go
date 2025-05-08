package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	AuthPermissionsVersion     = "1.0.5"
	AuthPermissionsDescription = "Create auth.permissions table for permission management"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[AuthPermissionsVersion] = &Migration{
		Version:     AuthPermissionsVersion,
		Description: AuthPermissionsDescription,
		DependsOn:   []string{"1.0.4"}, // Depends on auth.roles
		Up:          createAuthPermissionsTable,
		Down:        dropAuthPermissionsTable,
	}

	// Register the migration with Bun's migration system
	registerMigration(MigrationRegistry[AuthPermissionsVersion])
}

// createAuthPermissionsTable creates the auth.permissions table
func createAuthPermissionsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.0.5: Creating auth.permissions table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create the permissions table - for defining granular permissions
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS auth.permissions (
			id BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			name TEXT NOT NULL,
			description TEXT,
			resource TEXT NOT NULL,
			action TEXT NOT NULL,
			is_system BOOLEAN NOT NULL DEFAULT FALSE
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating permissions table: %w", err)
	}

	// Create indexes and constraints for permissions
	_, err = tx.ExecContext(ctx, `
		-- Unique constraint to prevent duplicate permissions
		CREATE UNIQUE INDEX IF NOT EXISTS idx_permissions_resource_action 
		ON auth.permissions(resource, action);
		
		-- Index for name lookups
		CREATE UNIQUE INDEX IF NOT EXISTS idx_permissions_name 
		ON auth.permissions(name);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for permissions table: %w", err)
	}

	// Create trigger for updating updated_at column
	_, err = tx.ExecContext(ctx, `
		-- Trigger for permissions
		DROP TRIGGER IF EXISTS update_permissions_updated_at ON auth.permissions;
		CREATE TRIGGER update_permissions_updated_at
		BEFORE UPDATE ON auth.permissions
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger for permissions: %w", err)
	}

	// Insert default system permissions
	_, err = tx.ExecContext(ctx, `
		INSERT INTO auth.permissions (name, description, resource, action, is_system)
		VALUES 
			('user.create', 'Create new users', 'user', 'create', TRUE),
			('user.read', 'View user information', 'user', 'read', TRUE),
			('user.update', 'Update user information', 'user', 'update', TRUE),
			('user.delete', 'Delete users', 'user', 'delete', TRUE),
			
			('role.create', 'Create new roles', 'role', 'create', TRUE),
			('role.read', 'View role information', 'role', 'read', TRUE),
			('role.update', 'Update role information', 'role', 'update', TRUE),
			('role.delete', 'Delete roles', 'role', 'delete', TRUE),
			
			('permission.create', 'Create new permissions', 'permission', 'create', TRUE),
			('permission.read', 'View permission information', 'permission', 'read', TRUE),
			('permission.update', 'Update permission information', 'permission', 'update', TRUE),
			('permission.delete', 'Delete permissions', 'permission', 'delete', TRUE),
			
			('system.manage', 'Manage system settings', 'system', 'manage', TRUE)
		ON CONFLICT (name) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("error inserting default permissions: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// dropAuthPermissionsTable drops the auth.permissions table
func dropAuthPermissionsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.0.5: Removing auth.permissions table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop trigger first
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_permissions_updated_at ON auth.permissions;
	`)
	if err != nil {
		return fmt.Errorf("error dropping trigger for permissions table: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS auth.permissions CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping auth.permissions table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
