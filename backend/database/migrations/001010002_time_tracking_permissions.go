package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	timeTrackingPermissionsVersion     = "1.10.2"
	timeTrackingPermissionsDescription = "Add time tracking permissions for all staff roles"
)

func init() {
	MigrationRegistry[timeTrackingPermissionsVersion] = &Migration{
		Version:     timeTrackingPermissionsVersion,
		Description: timeTrackingPermissionsDescription,
		DependsOn:   []string{"1.0.5", "1.0.4"}, // Depends on auth.permissions and auth.roles
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addTimeTrackingPermissions(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return removeTimeTrackingPermissions(ctx, db)
		},
	)
}

func addTimeTrackingPermissions(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.10.2: Adding time tracking permissions...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Insert time_tracking:own permission
	_, err = tx.ExecContext(ctx, `
		INSERT INTO auth.permissions (name, description, resource, action)
		VALUES
			('time_tracking:own', 'Track own working time', 'time_tracking', 'own')
		ON CONFLICT (name) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("error inserting time tracking permissions: %w", err)
	}

	// Grant time_tracking:own to staff-related roles only (not guest/guardian)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO auth.role_permissions (role_id, permission_id)
		SELECT r.id, p.id
		FROM auth.roles r
		CROSS JOIN auth.permissions p
		WHERE p.name = 'time_tracking:own'
		  AND r.name IN ('admin', 'user', 'teacher', 'staff')
		ON CONFLICT (role_id, permission_id) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("error granting time tracking permissions to roles: %w", err)
	}

	fmt.Println("Migration 1.10.2: Successfully added time tracking permissions")
	return tx.Commit()
}

func removeTimeTrackingPermissions(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.10.2: Removing time tracking permissions...")

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
			SELECT id FROM auth.permissions WHERE resource = 'time_tracking'
		)
	`)
	if err != nil {
		return fmt.Errorf("error removing role permissions: %w", err)
	}

	// Remove permissions
	_, err = tx.ExecContext(ctx, `
		DELETE FROM auth.permissions WHERE resource = 'time_tracking'
	`)
	if err != nil {
		return fmt.Errorf("error removing time tracking permissions: %w", err)
	}

	fmt.Println("Migration 1.10.2: Successfully removed time tracking permissions")
	return tx.Commit()
}
