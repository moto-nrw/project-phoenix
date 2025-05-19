package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	FixPermissionNamesVersion     = "1.5.3"
	FixPermissionNamesDescription = "Fix permission names to match backend constants"
)

// init registers this migration when the package is loaded
func init() {
	// Register migration with explicit version
	MigrationRegistry[FixPermissionNamesVersion] = &Migration{
		Version:     FixPermissionNamesVersion,
		Description: FixPermissionNamesDescription,
		DependsOn:   []string{"1.0.6"}, // Depends on auth.role_permissions table
	}

	// Register the migration functions
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return fixPermissionNames(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return revertPermissionNames(ctx, db)
		},
	)
}

// fixPermissionNames updates permission names to match the backend constants
func fixPermissionNames(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.5.3: Fixing permission names to match backend constants...")

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

	// Update existing permission names to match backend constants
	// The backend uses plural resource names and colon separators
	_, err = tx.ExecContext(ctx, `
		UPDATE auth.permissions 
		SET 
			name = CASE name
				-- User permissions
				WHEN 'user.create' THEN 'users:create'
				WHEN 'user.read' THEN 'users:read'
				WHEN 'user.update' THEN 'users:update'
				WHEN 'user.delete' THEN 'users:delete'
				
				-- Role permissions (already using plural)
				WHEN 'role.create' THEN 'roles:create'
				WHEN 'role.read' THEN 'roles:read'
				WHEN 'role.update' THEN 'roles:update'
				WHEN 'role.delete' THEN 'roles:delete'
				
				-- Permission permissions (already using plural)
				WHEN 'permission.create' THEN 'permissions:create'
				WHEN 'permission.read' THEN 'permissions:read'
				WHEN 'permission.update' THEN 'permissions:update'
				WHEN 'permission.delete' THEN 'permissions:delete'
				
				-- System permissions
				WHEN 'system.manage' THEN 'system:manage'
				
				ELSE name
			END,
			resource = CASE resource
				WHEN 'user' THEN 'users'
				WHEN 'role' THEN 'roles'
				WHEN 'permission' THEN 'permissions'
				ELSE resource
			END
		WHERE name LIKE '%.%'
	`)
	if err != nil {
		return fmt.Errorf("error updating permission names: %w", err)
	}

	// Update role_permissions table to use new permission names
	_, err = tx.ExecContext(ctx, `
		UPDATE auth.role_permissions rp
		SET permission_id = p_new.id
		FROM auth.permissions p_old
		JOIN auth.permissions p_new ON 
			(p_old.name = 'user.read' AND p_new.name = 'users:read')
		WHERE rp.permission_id = p_old.id
	`)
	if err != nil {
		return fmt.Errorf("error updating role_permissions: %w", err)
	}

	// Add additional permissions that may be needed
	_, err = tx.ExecContext(ctx, `
		INSERT INTO auth.permissions (name, description, resource, action, is_system)
		VALUES 
			-- Activity permissions
			('activities:create', 'Create new activities', 'activities', 'create', TRUE),
			('activities:read', 'View activity information', 'activities', 'read', TRUE),
			('activities:update', 'Update activity information', 'activities', 'update', TRUE),
			('activities:delete', 'Delete activities', 'activities', 'delete', TRUE),
			('activities:list', 'List activities', 'activities', 'list', TRUE),
			('activities:manage', 'Manage all aspects of activities', 'activities', 'manage', TRUE),
			('activities:enroll', 'Enroll students in activities', 'activities', 'enroll', TRUE),
			('activities:assign', 'Assign supervisors to activities', 'activities', 'assign', TRUE),
			
			-- Room permissions
			('rooms:create', 'Create new rooms', 'rooms', 'create', TRUE),
			('rooms:read', 'View room information', 'rooms', 'read', TRUE),
			('rooms:update', 'Update room information', 'rooms', 'update', TRUE),
			('rooms:delete', 'Delete rooms', 'rooms', 'delete', TRUE),
			('rooms:list', 'List rooms', 'rooms', 'list', TRUE),
			('rooms:manage', 'Manage all aspects of rooms', 'rooms', 'manage', TRUE),
			
			-- Group permissions
			('groups:create', 'Create new groups', 'groups', 'create', TRUE),
			('groups:read', 'View group information', 'groups', 'read', TRUE),
			('groups:update', 'Update group information', 'groups', 'update', TRUE),
			('groups:delete', 'Delete groups', 'groups', 'delete', TRUE),
			('groups:list', 'List groups', 'groups', 'list', TRUE),
			('groups:manage', 'Manage all aspects of groups', 'groups', 'manage', TRUE),
			('groups:assign', 'Assign students to groups', 'groups', 'assign', TRUE),
			
			-- Config permissions
			('config:read', 'Read configuration settings', 'config', 'read', TRUE),
			('config:update', 'Update configuration settings', 'config', 'update', TRUE),
			('config:manage', 'Manage all configuration settings', 'config', 'manage', TRUE),
			
			-- Auth permissions
			('auth:manage', 'Manage authentication system', 'auth', 'manage', TRUE),
			
			-- Visit permissions  
			('visits:create', 'Create new visits', 'visits', 'create', TRUE),
			('visits:read', 'View visit information', 'visits', 'read', TRUE),
			('visits:update', 'Update visit information', 'visits', 'update', TRUE),
			('visits:delete', 'Delete visits', 'visits', 'delete', TRUE),
			('visits:list', 'List visits', 'visits', 'list', TRUE),
			('visits:manage', 'Manage all aspects of visits', 'visits', 'manage', TRUE)
		ON CONFLICT (name) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("error inserting additional permissions: %w", err)
	}

	// Grant appropriate permissions to user role for teacher creation
	// Teachers need to be able to create persons when creating new teacher accounts
	_, err = tx.ExecContext(ctx, `
		INSERT INTO auth.role_permissions (role_id, permission_id)
		SELECT r.id, p.id
		FROM auth.roles r
		CROSS JOIN auth.permissions p
		WHERE r.name = 'user' 
		AND p.name IN ('users:create', 'users:update')
		ON CONFLICT (role_id, permission_id) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("error granting permissions to user role: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// revertPermissionNames reverts permission names back to dot notation
func revertPermissionNames(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.5.3: Reverting permission names...")

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

	// Revert permission names back to dot notation
	_, err = tx.ExecContext(ctx, `
		UPDATE auth.permissions 
		SET 
			name = CASE name
				-- User permissions
				WHEN 'users:create' THEN 'user.create'
				WHEN 'users:read' THEN 'user.read'
				WHEN 'users:update' THEN 'user.update'
				WHEN 'users:delete' THEN 'user.delete'
				
				-- Role permissions
				WHEN 'roles:create' THEN 'role.create'
				WHEN 'roles:read' THEN 'role.read'
				WHEN 'roles:update' THEN 'role.update'
				WHEN 'roles:delete' THEN 'role.delete'
				
				-- Permission permissions
				WHEN 'permissions:create' THEN 'permission.create'
				WHEN 'permissions:read' THEN 'permission.read'
				WHEN 'permissions:update' THEN 'permission.update'
				WHEN 'permissions:delete' THEN 'permission.delete'
				
				-- System permissions
				WHEN 'system:manage' THEN 'system.manage'
				
				ELSE name
			END,
			resource = CASE resource
				WHEN 'users' THEN 'user'
				WHEN 'roles' THEN 'role'
				WHEN 'permissions' THEN 'permission'
				ELSE resource
			END
		WHERE name LIKE '%:%'
	`)
	if err != nil {
		return fmt.Errorf("error reverting permission names: %w", err)
	}

	// Remove the additional permissions added by this migration
	_, err = tx.ExecContext(ctx, `
		DELETE FROM auth.permissions
		WHERE name IN (
			'activities:create', 'activities:read', 'activities:update', 'activities:delete', 
			'activities:list', 'activities:manage', 'activities:enroll', 'activities:assign',
			'rooms:create', 'rooms:read', 'rooms:update', 'rooms:delete', 
			'rooms:list', 'rooms:manage',
			'groups:create', 'groups:read', 'groups:update', 'groups:delete', 
			'groups:list', 'groups:manage', 'groups:assign',
			'config:read', 'config:update', 'config:manage',
			'auth:manage',
			'visits:create', 'visits:read', 'visits:update', 'visits:delete', 
			'visits:list', 'visits:manage'
		)
	`)
	if err != nil {
		return fmt.Errorf("error removing additional permissions: %w", err)
	}

	// Remove permissions from user role
	_, err = tx.ExecContext(ctx, `
		DELETE FROM auth.role_permissions
		WHERE role_id IN (SELECT id FROM auth.roles WHERE name = 'user')
		AND permission_id IN (
			SELECT id FROM auth.permissions 
			WHERE name IN ('users:create', 'users:update')
		)
	`)
	if err != nil {
		return fmt.Errorf("error removing permissions from user role: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}