package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	AuthRolePermissionsVersion     = "1.0.6"
	AuthRolePermissionsDescription = "Create auth.role_permissions table for role-permission mapping"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[AuthRolePermissionsVersion] = &Migration{
		Version:     AuthRolePermissionsVersion,
		Description: AuthRolePermissionsDescription,
		DependsOn:   []string{"1.0.4", "1.0.5"}, // Depends on auth.roles and auth.permissions
		Up:          createAuthRolePermissionsTable,
		Down:        dropAuthRolePermissionsTable,
	}

	// Register the migration with Bun's migration system
	registerMigration(MigrationRegistry[AuthRolePermissionsVersion])
}

// createAuthRolePermissionsTable creates the auth.role_permissions table
func createAuthRolePermissionsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.0.6: Creating auth.role_permissions table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create the role_permissions table - for mapping roles to permissions
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS auth.role_permissions (
			id BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			role_id BIGINT NOT NULL,
			permission_id BIGINT NOT NULL,
			
			CONSTRAINT fk_role_permissions_role 
				FOREIGN KEY (role_id) 
				REFERENCES auth.roles(id) 
				ON DELETE CASCADE,
				
			CONSTRAINT fk_role_permissions_permission 
				FOREIGN KEY (permission_id) 
				REFERENCES auth.permissions(id) 
				ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating role_permissions table: %w", err)
	}

	// Create indexes for role_permissions
	_, err = tx.ExecContext(ctx, `
		-- Unique constraint to prevent duplicate role-permission mappings
		CREATE UNIQUE INDEX IF NOT EXISTS idx_role_permissions_role_permission 
		ON auth.role_permissions(role_id, permission_id);
		
		-- Index for efficient role lookups
		CREATE INDEX IF NOT EXISTS idx_role_permissions_role_id 
		ON auth.role_permissions(role_id);
		
		-- Index for efficient permission lookups
		CREATE INDEX IF NOT EXISTS idx_role_permissions_permission_id 
		ON auth.role_permissions(permission_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for role_permissions table: %w", err)
	}

	// Create trigger for updating updated_at column
	_, err = tx.ExecContext(ctx, `
		-- Trigger for role_permissions
		DROP TRIGGER IF EXISTS update_role_permissions_updated_at ON auth.role_permissions;
		CREATE TRIGGER update_role_permissions_updated_at
		BEFORE UPDATE ON auth.role_permissions
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger for role_permissions: %w", err)
	}

	// Assign all permissions to admin role
	_, err = tx.ExecContext(ctx, `
		-- Get the admin role ID
		WITH admin_role AS (
			SELECT id FROM auth.roles WHERE name = 'admin' LIMIT 1
		)
		INSERT INTO auth.role_permissions (role_id, permission_id)
		SELECT admin_role.id, p.id
		FROM auth.permissions p, admin_role
		ON CONFLICT (role_id, permission_id) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("error assigning permissions to admin role: %w", err)
	}

	// Assign basic permissions to user role
	_, err = tx.ExecContext(ctx, `
		-- Get the user role ID
		WITH user_role AS (
			SELECT id FROM auth.roles WHERE name = 'user' LIMIT 1
		)
		INSERT INTO auth.role_permissions (role_id, permission_id)
		SELECT user_role.id, p.id
		FROM auth.permissions p, user_role
		WHERE p.name IN ('user.read')
		ON CONFLICT (role_id, permission_id) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("error assigning permissions to user role: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// dropAuthRolePermissionsTable drops the auth.role_permissions table
func dropAuthRolePermissionsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.0.6: Removing auth.role_permissions table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop trigger first
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_role_permissions_updated_at ON auth.role_permissions;
	`)
	if err != nil {
		return fmt.Errorf("error dropping trigger for role_permissions table: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS auth.role_permissions CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping auth.role_permissions table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
