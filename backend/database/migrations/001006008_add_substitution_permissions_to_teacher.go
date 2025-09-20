package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	AddSubstitutionPermissionsVersion     = "1.6.8"
	AddSubstitutionPermissionsDescription = "Add substitution permissions to teacher and staff roles"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[AddSubstitutionPermissionsVersion] = &Migration{
		Version:     AddSubstitutionPermissionsVersion,
		Description: AddSubstitutionPermissionsDescription,
		DependsOn:   []string{"1.2.1", "1.2.2", "1.2.4", "1.5.3"}, // Depends on roles, permissions, role_permissions, and fix_permission_names
	}

	// Register the actual migration functions
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addSubstitutionPermissionsToRoles(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return removeSubstitutionPermissionsFromRoles(ctx, db)
		},
	)
}

func addSubstitutionPermissionsToRoles(ctx context.Context, db *bun.DB) error {
	log.Println("Adding substitution permissions to teacher and staff roles...")

	// Begin a transaction
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Track whether transaction was committed
	committed := false
	defer func() {
		if !committed {
			if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
				log.Printf("Failed to rollback transaction: %v", rbErr)
			}
		}
	}()

	// Add substitution permissions to teacher role
	query := `
	INSERT INTO auth.role_permissions (role_id, permission_id, created_at, updated_at)
	SELECT 
		r.id as role_id,
		p.id as permission_id,
		NOW() as created_at,
		NOW() as updated_at
	FROM auth.roles r
	CROSS JOIN auth.permissions p
	WHERE r.name = 'teacher'
	AND p.name IN (
		'substitutions:read',
		'substitutions:create',
		'substitutions:update',
		'substitutions:delete'
	)
	ON CONFLICT (role_id, permission_id) DO NOTHING;
	`

	_, err = tx.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to add substitution permissions to teacher role: %w", err)
	}

	// Also add substitution permissions to staff role (if exists)
	staffQuery := `
	INSERT INTO auth.role_permissions (role_id, permission_id, created_at, updated_at)
	SELECT 
		r.id as role_id,
		p.id as permission_id,
		NOW() as created_at,
		NOW() as updated_at
	FROM auth.roles r
	CROSS JOIN auth.permissions p
	WHERE r.name = 'staff'
	AND p.name IN (
		'substitutions:read',
		'substitutions:create',
		'substitutions:update',
		'substitutions:delete'
	)
	ON CONFLICT (role_id, permission_id) DO NOTHING;
	`

	_, err = tx.ExecContext(ctx, staffQuery)
	if err != nil {
		// Staff role might not exist, which is okay
		log.Printf("Note: Could not add permissions to staff role (role might not exist): %v", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	committed = true

	log.Println("Successfully added substitution permissions to teacher and staff roles")
	return nil
}

func removeSubstitutionPermissionsFromRoles(ctx context.Context, db *bun.DB) error {
	log.Println("Removing substitution permissions from teacher and staff roles...")

	// Remove substitution permissions from teacher role
	_, err := db.ExecContext(ctx, `
		DELETE FROM auth.role_permissions
		WHERE role_id IN (SELECT id FROM auth.roles WHERE name = 'teacher')
		AND permission_id IN (
			SELECT id FROM auth.permissions 
			WHERE name IN (
				'substitutions:read',
				'substitutions:create',
				'substitutions:update',
				'substitutions:delete'
			)
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to remove substitution permissions from teacher role: %w", err)
	}

	// Remove substitution permissions from staff role
	_, err = db.ExecContext(ctx, `
		DELETE FROM auth.role_permissions
		WHERE role_id IN (SELECT id FROM auth.roles WHERE name = 'staff')
		AND permission_id IN (
			SELECT id FROM auth.permissions 
			WHERE name IN (
				'substitutions:read',
				'substitutions:create',
				'substitutions:update',
				'substitutions:delete'
			)
		);
	`)
	if err != nil {
		// Staff role might not exist, which is okay
		log.Printf("Note: Could not remove permissions from staff role (role might not exist): %v", err)
	}

	log.Println("Successfully removed substitution permissions from teacher and staff roles")
	return nil
}
