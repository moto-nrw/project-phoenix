package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	suggestionsPermissionsVersion     = "1.9.2"
	suggestionsPermissionsDescription = "Add suggestions permissions for teacher and admin roles"
)

func init() {
	MigrationRegistry[suggestionsPermissionsVersion] = &Migration{
		Version:     suggestionsPermissionsVersion,
		Description: suggestionsPermissionsDescription,
		DependsOn:   []string{"1.0.5", "1.0.4"}, // Depends on auth.permissions and auth.roles
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addSuggestionsPermissions(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return removeSuggestionsPermissions(ctx, db)
		},
	)
}

func addSuggestionsPermissions(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.9.2: Adding suggestions permissions...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Insert suggestions permissions
	_, err = tx.ExecContext(ctx, `
		INSERT INTO auth.permissions (name, description, resource, action)
		VALUES
			('suggestions:create', 'Create suggestions', 'suggestions', 'create'),
			('suggestions:read', 'View suggestions', 'suggestions', 'read'),
			('suggestions:update', 'Update suggestions', 'suggestions', 'update'),
			('suggestions:delete', 'Delete suggestions', 'suggestions', 'delete'),
			('suggestions:list', 'List suggestions', 'suggestions', 'list'),
			('suggestions:manage', 'Full control over suggestions', 'suggestions', 'manage')
		ON CONFLICT (name) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("error inserting suggestions permissions: %w", err)
	}

	// Grant suggestions permissions to teacher and admin roles
	_, err = tx.ExecContext(ctx, `
		INSERT INTO auth.role_permissions (role_id, permission_id)
		SELECT r.id, p.id
		FROM auth.roles r
		CROSS JOIN auth.permissions p
		WHERE r.name IN ('admin', 'teacher')
		  AND p.resource = 'suggestions'
		ON CONFLICT (role_id, permission_id) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("error granting suggestions permissions to roles: %w", err)
	}

	fmt.Println("Migration 1.9.2: Successfully added suggestions permissions")
	return tx.Commit()
}

func removeSuggestionsPermissions(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.9.2: Removing suggestions permissions...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Remove role_permissions first (foreign key)
	_, err = tx.ExecContext(ctx, `
		DELETE FROM auth.role_permissions
		WHERE permission_id IN (
			SELECT id FROM auth.permissions WHERE resource = 'suggestions'
		)
	`)
	if err != nil {
		return fmt.Errorf("error removing role permissions: %w", err)
	}

	// Remove permissions
	_, err = tx.ExecContext(ctx, `
		DELETE FROM auth.permissions WHERE resource = 'suggestions'
	`)
	if err != nil {
		return fmt.Errorf("error removing suggestions permissions: %w", err)
	}

	fmt.Println("Migration 1.9.2: Successfully removed suggestions permissions")
	return tx.Commit()
}
