package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	GradeTransitionPermissionsVersion     = "1.7.7"
	GradeTransitionPermissionsDescription = "Add grade transition permissions to admin role"
)

func init() {
	MigrationRegistry[GradeTransitionPermissionsVersion] = &Migration{
		Version:     GradeTransitionPermissionsVersion,
		Description: GradeTransitionPermissionsDescription,
		DependsOn:   []string{"1.0.5", "1.0.4"}, // Depends on auth.permissions and auth.roles
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addGradeTransitionPermissions(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return removeGradeTransitionPermissions(ctx, db)
		},
	)
}

// addGradeTransitionPermissions adds grade transition permissions and grants them to admin role
func addGradeTransitionPermissions(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.7.7: Adding grade transition permissions...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Insert grade transition permissions
	_, err = tx.ExecContext(ctx, `
		INSERT INTO auth.permissions (name, description, resource, action)
		VALUES
			('grade_transitions:read', 'View grade transitions', 'grade_transitions', 'read'),
			('grade_transitions:create', 'Create grade transitions', 'grade_transitions', 'create'),
			('grade_transitions:update', 'Update grade transitions', 'grade_transitions', 'update'),
			('grade_transitions:delete', 'Delete grade transitions', 'grade_transitions', 'delete'),
			('grade_transitions:apply', 'Apply/revert grade transitions', 'grade_transitions', 'apply')
		ON CONFLICT (name) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("error inserting grade transition permissions: %w", err)
	}

	// Grant grade transition permissions to admin role
	_, err = tx.ExecContext(ctx, `
		INSERT INTO auth.role_permissions (role_id, permission_id)
		SELECT r.id, p.id
		FROM auth.roles r
		CROSS JOIN auth.permissions p
		WHERE r.name = 'admin'
		  AND p.resource = 'grade_transitions'
		ON CONFLICT (role_id, permission_id) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("error granting grade transition permissions to admin role: %w", err)
	}

	fmt.Println("Migration 1.7.7: Successfully added grade transition permissions")
	return tx.Commit()
}

// removeGradeTransitionPermissions removes grade transition permissions
func removeGradeTransitionPermissions(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.7.7: Removing grade transition permissions...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Remove role_permissions first (due to foreign key)
	_, err = tx.ExecContext(ctx, `
		DELETE FROM auth.role_permissions
		WHERE permission_id IN (
			SELECT id FROM auth.permissions WHERE resource = 'grade_transitions'
		)
	`)
	if err != nil {
		return fmt.Errorf("error removing role permissions: %w", err)
	}

	// Remove permissions
	_, err = tx.ExecContext(ctx, `
		DELETE FROM auth.permissions WHERE resource = 'grade_transitions'
	`)
	if err != nil {
		return fmt.Errorf("error removing grade transition permissions: %w", err)
	}

	fmt.Println("Migration 1.7.7: Successfully removed grade transition permissions")
	return tx.Commit()
}
