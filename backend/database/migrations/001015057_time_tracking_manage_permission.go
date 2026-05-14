package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	timeTrackingManagePermissionVersion     = "1.15.57"
	timeTrackingManagePermissionDescription = "Add time_tracking:manage permission for admin cross-staff edits"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     timeTrackingManagePermissionVersion,
		Description: timeTrackingManagePermissionDescription,
		// Builds on 1.10.2 (time_tracking:own) and 1.0.5/1.0.4 (auth tables).
		DependsOn: []string{"1.10.2"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addTimeTrackingManagePermission(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return removeTimeTrackingManagePermission(ctx, db)
		},
	)
}

// addTimeTrackingManagePermission introduces time_tracking:manage and grants
// it to the admin role only. Without this split a Betreuer with
// time_tracking:own would be indistinguishable from an OGS-Leitung at the
// permission layer — both could edit work sessions, just on different scopes.
// time_tracking:manage is the gate for cross-staff edits and creating
// nachgetragene sessions.
func addTimeTrackingManagePermission(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.57: Adding time_tracking:manage permission...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO auth.permissions (name, description, resource, action)
		VALUES
			('time_tracking:manage', 'Manage other staff working time (admin)', 'time_tracking', 'manage')
		ON CONFLICT (name) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("error inserting time_tracking:manage: %w", err)
	}

	// Admin only. Betreuer/Teacher/Staff explicitly do NOT get this — they
	// already have time_tracking:own for their own sessions.
	_, err = tx.ExecContext(ctx, `
		INSERT INTO auth.role_permissions (role_id, permission_id)
		SELECT r.id, p.id
		FROM auth.roles r
		CROSS JOIN auth.permissions p
		WHERE p.name = 'time_tracking:manage'
		  AND r.name = 'admin'
		ON CONFLICT (role_id, permission_id) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("error granting time_tracking:manage to admin: %w", err)
	}

	fmt.Println("Migration 1.15.57: Successfully granted time_tracking:manage to admin role")
	return tx.Commit()
}

func removeTimeTrackingManagePermission(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.57: Removing time_tracking:manage...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		DELETE FROM auth.role_permissions
		WHERE permission_id IN (
			SELECT id FROM auth.permissions WHERE name = 'time_tracking:manage'
		)
	`)
	if err != nil {
		return fmt.Errorf("error removing role permissions: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		DELETE FROM auth.permissions WHERE name = 'time_tracking:manage'
	`)
	if err != nil {
		return fmt.Errorf("error removing permission: %w", err)
	}

	fmt.Println("Migration 1.15.57: Successfully rolled back")
	return tx.Commit()
}
