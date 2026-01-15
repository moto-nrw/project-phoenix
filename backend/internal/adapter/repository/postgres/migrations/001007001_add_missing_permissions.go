package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	AddMissingPermissionsVersion     = "1.7.1"
	AddMissingPermissionsDescription = "Add missing substitutions and users:manage permissions"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[AddMissingPermissionsVersion] = &Migration{
		Version:     AddMissingPermissionsVersion,
		Description: AddMissingPermissionsDescription,
		DependsOn:   []string{"1.7.0"}, // Depends on guest_permissions
	}

	// Register the migration functions
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addMissingPermissions(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return removeMissingPermissions(ctx, db)
		},
	)
}

// addMissingPermissions adds substitutions permissions and users:manage
func addMissingPermissions(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.7.1: Adding missing permissions (substitutions, users:manage)...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && err == nil {
			err = rbErr
		}
	}()

	// Add missing permissions
	_, err = tx.ExecContext(ctx, `
		INSERT INTO auth.permissions (name, description, resource, action, is_system)
		VALUES
			-- Substitutions permissions (matching backend constants)
			('substitutions:create', 'Create substitution entries', 'substitutions', 'create', TRUE),
			('substitutions:read', 'View substitution information', 'substitutions', 'read', TRUE),
			('substitutions:update', 'Update substitution entries', 'substitutions', 'update', TRUE),
			('substitutions:delete', 'Delete substitution entries', 'substitutions', 'delete', TRUE),
			('substitutions:list', 'List substitution entries', 'substitutions', 'list', TRUE),
			('substitutions:manage', 'Manage all aspects of substitutions', 'substitutions', 'manage', TRUE),

			-- Missing users permission
			('users:manage', 'Manage all aspects of users', 'users', 'manage', TRUE)
		ON CONFLICT (name) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("error inserting missing permissions: %w", err)
	}

	// Grant new permissions to admin role (via wildcard they already have access,
	// but this ensures they appear in the role_permissions table for consistency)
	_, err = tx.ExecContext(ctx, `
		WITH admin_role AS (
			SELECT id FROM auth.roles WHERE name = 'admin' LIMIT 1
		),
		new_permissions AS (
			SELECT id FROM auth.permissions
			WHERE name IN (
				'substitutions:create', 'substitutions:read', 'substitutions:update',
				'substitutions:delete', 'substitutions:list', 'substitutions:manage',
				'users:manage'
			)
		)
		INSERT INTO auth.role_permissions (role_id, permission_id)
		SELECT ar.id, np.id
		FROM admin_role ar
		CROSS JOIN new_permissions np
		ON CONFLICT (role_id, permission_id) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("error granting permissions to admin role: %w", err)
	}

	// Grant substitutions permissions to teacher role (they manage their own substitutions)
	_, err = tx.ExecContext(ctx, `
		WITH teacher_role AS (
			SELECT id FROM auth.roles WHERE name = 'teacher' LIMIT 1
		),
		substitution_permissions AS (
			SELECT id FROM auth.permissions
			WHERE name IN (
				'substitutions:create', 'substitutions:read', 'substitutions:update',
				'substitutions:delete', 'substitutions:list', 'substitutions:manage'
			)
		)
		INSERT INTO auth.role_permissions (role_id, permission_id)
		SELECT tr.id, sp.id
		FROM teacher_role tr
		CROSS JOIN substitution_permissions sp
		ON CONFLICT (role_id, permission_id) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("error granting substitution permissions to teacher role: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	fmt.Println("Migration 1.7.1: Successfully added missing permissions")
	return nil
}

// removeMissingPermissions removes the permissions added by this migration
func removeMissingPermissions(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.7.1: Removing added permissions...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && err == nil {
			err = rbErr
		}
	}()

	// Remove role_permissions first (foreign key constraint)
	_, err = tx.ExecContext(ctx, `
		DELETE FROM auth.role_permissions
		WHERE permission_id IN (
			SELECT id FROM auth.permissions
			WHERE name IN (
				'substitutions:create', 'substitutions:read', 'substitutions:update',
				'substitutions:delete', 'substitutions:list', 'substitutions:manage',
				'users:manage'
			)
		)
	`)
	if err != nil {
		return fmt.Errorf("error removing role_permissions: %w", err)
	}

	// Remove permissions
	_, err = tx.ExecContext(ctx, `
		DELETE FROM auth.permissions
		WHERE name IN (
			'substitutions:create', 'substitutions:read', 'substitutions:update',
			'substitutions:delete', 'substitutions:list', 'substitutions:manage',
			'users:manage'
		)
	`)
	if err != nil {
		return fmt.Errorf("error removing permissions: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	fmt.Println("Migration 1.7.1: Successfully removed added permissions")
	return nil
}
