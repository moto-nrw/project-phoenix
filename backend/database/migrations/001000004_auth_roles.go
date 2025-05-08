package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	AuthRolesVersion     = "1.1.0"
	AuthRolesDescription = "Create auth.roles table for role management"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[AuthRolesVersion] = &Migration{
		Version:     AuthRolesVersion,
		Description: AuthRolesDescription,
		DependsOn:   []string{"1.0.1"}, // Depends on auth.accounts
		Up:          createAuthRolesTable,
		Down:        dropAuthRolesTable,
	}

	// Register the migration with Bun's migration system
	registerMigration(MigrationRegistry[AuthRolesVersion])
}

// createAuthRolesTable creates the auth.roles table
func createAuthRolesTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.1.0: Creating auth.roles table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create the roles table - for defining application roles
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS auth.roles (
			id BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			name TEXT NOT NULL,
			description TEXT,
			is_system BOOLEAN NOT NULL DEFAULT FALSE,
			metadata JSONB
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating roles table: %w", err)
	}

	// Create indexes for roles
	_, err = tx.ExecContext(ctx, `
		CREATE UNIQUE INDEX IF NOT EXISTS idx_roles_name ON auth.roles(name);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for roles table: %w", err)
	}

	// Create trigger for updating updated_at column
	_, err = tx.ExecContext(ctx, `
		-- Trigger for roles
		DROP TRIGGER IF EXISTS update_roles_updated_at ON auth.roles;
		CREATE TRIGGER update_roles_updated_at
		BEFORE UPDATE ON auth.roles
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger for roles: %w", err)
	}

	// Insert default roles
	_, err = tx.ExecContext(ctx, `
		INSERT INTO auth.roles (name, description, is_system)
		VALUES 
			('admin', 'System administrator with full access', TRUE),
			('user', 'Standard user with basic permissions', TRUE),
			('guest', 'Limited access for unauthenticated users', TRUE)
		ON CONFLICT (name) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("error inserting default roles: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// dropAuthRolesTable drops the auth.roles table
func dropAuthRolesTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.1.0: Removing auth.roles table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop trigger first
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_roles_updated_at ON auth.roles;
	`)
	if err != nil {
		return fmt.Errorf("error dropping trigger for roles table: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS auth.roles CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping auth.roles table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
