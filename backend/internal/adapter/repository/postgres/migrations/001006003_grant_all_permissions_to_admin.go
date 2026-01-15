package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	GrantAllPermissionsToAdminVersion     = "1.6.3"
	GrantAllPermissionsToAdminDescription = "Grant all permissions to admin role, including groups permissions"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[GrantAllPermissionsToAdminVersion] = &Migration{
		Version:     GrantAllPermissionsToAdminVersion,
		Description: GrantAllPermissionsToAdminDescription,
		DependsOn:   []string{"1.5.3", "1.6.2"}, // Depends on fix_permission_names and create_admin_account
	}

	// Register the actual migration functions
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return grantAllPermissionsToAdmin(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			// Rollback is a no-op since we're just adding permissions
			// The admin should keep whatever permissions they had before
			return nil
		},
	)
}

// grantAllPermissionsToAdmin ensures the admin role has all system permissions
func grantAllPermissionsToAdmin(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.6.3: Granting all permissions to admin role...")

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

	// First, ensure all permissions exist in the database
	// This includes any that might have been missed in previous migrations
	_, err = tx.ExecContext(ctx, `
		INSERT INTO auth.permissions (name, description, resource, action, is_system)
		VALUES 
			-- Groups permissions (these are the ones that were missing)
			('groups:create', 'Create new groups', 'groups', 'create', TRUE),
			('groups:read', 'View group information', 'groups', 'read', TRUE),
			('groups:update', 'Update group information', 'groups', 'update', TRUE),
			('groups:delete', 'Delete groups', 'groups', 'delete', TRUE),
			('groups:list', 'List groups', 'groups', 'list', TRUE),
			('groups:manage', 'Manage all aspects of groups', 'groups', 'manage', TRUE),
			('groups:assign', 'Assign students to groups', 'groups', 'assign', TRUE),
			
			-- Schedules permissions (ensuring they exist)
			('schedules:create', 'Create new schedules', 'schedules', 'create', TRUE),
			('schedules:read', 'View schedule information', 'schedules', 'read', TRUE),
			('schedules:update', 'Update schedule information', 'schedules', 'update', TRUE),
			('schedules:delete', 'Delete schedules', 'schedules', 'delete', TRUE),
			('schedules:list', 'List schedules', 'schedules', 'list', TRUE),
			('schedules:manage', 'Manage all aspects of schedules', 'schedules', 'manage', TRUE),
			
			-- IOT permissions
			('iot:read', 'View IoT device information', 'iot', 'read', TRUE),
			('iot:update', 'Update IoT device information', 'iot', 'update', TRUE),
			('iot:manage', 'Manage all aspects of IoT devices', 'iot', 'manage', TRUE),
			
			-- Feedback permissions
			('feedback:create', 'Create feedback entries', 'feedback', 'create', TRUE),
			('feedback:read', 'View feedback entries', 'feedback', 'read', TRUE),
			('feedback:delete', 'Delete feedback entries', 'feedback', 'delete', TRUE),
			('feedback:list', 'List feedback entries', 'feedback', 'list', TRUE),
			('feedback:manage', 'Manage all aspects of feedback', 'feedback', 'manage', TRUE),
			
			-- Admin wildcard permission
			('admin:*', 'Full system access', 'admin', '*', TRUE),
			('*:*', 'Full access to all resources', '*', '*', TRUE)
		ON CONFLICT (name) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("error ensuring all permissions exist: %w", err)
	}

	// Grant all permissions to the admin role
	// This query finds all permissions that the admin role doesn't have yet
	// and grants them
	_, err = tx.ExecContext(ctx, `
		WITH admin_role AS (
			SELECT id FROM auth.roles WHERE name = 'admin' LIMIT 1
		),
		missing_permissions AS (
			SELECT p.id as permission_id, ar.id as role_id
			FROM auth.permissions p
			CROSS JOIN admin_role ar
			WHERE NOT EXISTS (
				SELECT 1 
				FROM auth.role_permissions rp 
				WHERE rp.role_id = ar.id 
				AND rp.permission_id = p.id
			)
		)
		INSERT INTO auth.role_permissions (role_id, permission_id)
		SELECT role_id, permission_id
		FROM missing_permissions
	`)
	if err != nil {
		return fmt.Errorf("error granting permissions to admin role: %w", err)
	}

	// Get count of permissions granted for logging
	var count int
	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM auth.role_permissions rp
		JOIN auth.roles r ON r.id = rp.role_id
		WHERE r.name = 'admin'
	`).Scan(&count)
	if err != nil {
		return fmt.Errorf("error counting admin permissions: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	fmt.Printf("Admin role now has %d permissions\n", count)
	fmt.Println("Migration 1.6.3 completed successfully")

	return nil
}
