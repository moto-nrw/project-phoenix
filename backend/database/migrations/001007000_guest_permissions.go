package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	GuestPermissionsVersion     = "1.7.0"
	GuestPermissionsDescription = "Assign users:read and users:list permissions to guest role"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[GuestPermissionsVersion] = &Migration{
		Version:     GuestPermissionsVersion,
		Description: GuestPermissionsDescription,
		DependsOn:   []string{"1.0.4", "1.0.5", "1.0.6"}, // Depends on auth.roles, auth.permissions, auth.role_permissions
	}

	// Migration 1.7.0: Assign guest permissions
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return assignGuestPermissions(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return removeGuestPermissions(ctx, db)
		},
	)
}

// assignGuestPermissions assigns users:read and users:list permissions to the guest role
func assignGuestPermissions(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.7.0: Assigning permissions to guest role...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Failed to rollback transaction in guest permissions migration: %v", err)
		}
	}()

	// Ensure the required permissions exist (should already exist from seed, but be safe)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO auth.permissions (name, description, resource, action)
		VALUES
			('users:read', 'Permission to read user data', 'users', 'read'),
			('users:list', 'Permission to list users', 'users', 'list')
		ON CONFLICT (name) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("error ensuring permissions exist: %w", err)
	}

	// Assign permissions to guest role
	_, err = tx.ExecContext(ctx, `
		INSERT INTO auth.role_permissions (role_id, permission_id)
		SELECT r.id, p.id
		FROM auth.roles r
		CROSS JOIN auth.permissions p
		WHERE r.name = 'guest'
		AND p.name IN ('users:read', 'users:list')
		ON CONFLICT (role_id, permission_id) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("error assigning permissions to guest role: %w", err)
	}

	fmt.Println("Migration 1.7.0: Successfully assigned users:read and users:list to guest role")

	// Commit the transaction
	return tx.Commit()
}

// removeGuestPermissions removes the permission assignments from the guest role
func removeGuestPermissions(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.7.0: Removing permissions from guest role...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf("Failed to rollback transaction in guest permissions migration: %v", err)
		}
	}()

	// Remove the permission assignments from guest role
	_, err = tx.ExecContext(ctx, `
		DELETE FROM auth.role_permissions
		WHERE role_id = (SELECT id FROM auth.roles WHERE name = 'guest')
		AND permission_id IN (
			SELECT id FROM auth.permissions WHERE name IN ('users:read', 'users:list')
		)
	`)
	if err != nil {
		return fmt.Errorf("error removing permissions from guest role: %w", err)
	}

	// Note: We don't delete the permissions themselves (users:read, users:list)
	// because they may be used by other roles or created by seed data

	fmt.Println("Migration 1.7.0: Successfully removed guest role permissions")

	// Commit the transaction
	return tx.Commit()
}
