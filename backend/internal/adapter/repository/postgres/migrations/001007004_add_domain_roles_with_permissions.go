package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	AddDomainRolesVersion     = "1.7.4"
	AddDomainRolesDescription = "Add teacher, staff, guardian roles with domain-specific permissions"
)

func init() {
	MigrationRegistry[AddDomainRolesVersion] = &Migration{
		Version:     AddDomainRolesVersion,
		Description: AddDomainRolesDescription,
		DependsOn:   []string{"1.7.3"}, // Depends on fix_group_supervisor_unique_constraint
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addDomainRolesWithPermissions(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return removeDomainRolesWithPermissions(ctx, db)
		},
	)
}

// addDomainRolesWithPermissions adds teacher, staff, guardian roles with their permissions
// Safe for existing databases - uses ON CONFLICT DO NOTHING
func addDomainRolesWithPermissions(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.7.4: Adding domain roles (teacher, staff, guardian) with permissions...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && err == nil {
			err = rbErr
		}
	}()

	// Step 1: Add domain-specific roles (safe for existing DBs)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO auth.roles (name, description, is_system)
		VALUES
			('teacher', 'Teacher with group management capabilities', TRUE),
			('staff', 'General staff member with visit management', TRUE),
			('guardian', 'Parent or guardian with limited access to their children', TRUE)
		ON CONFLICT (name) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("error inserting domain roles: %w", err)
	}

	// Step 2: Ensure all required permissions exist
	_, err = tx.ExecContext(ctx, `
		INSERT INTO auth.permissions (name, description, resource, action, is_system)
		VALUES
			-- Visits permissions (critical for check-in/check-out)
			('visits:create', 'Create visit records', 'visits', 'create', TRUE),
			('visits:read', 'View visit information', 'visits', 'read', TRUE),
			('visits:update', 'Update visit records', 'visits', 'update', TRUE),
			('visits:delete', 'Delete visit records', 'visits', 'delete', TRUE),
			('visits:list', 'List visit records', 'visits', 'list', TRUE),
			('visits:manage', 'Full visits management', 'visits', 'manage', TRUE),

			-- Groups permissions
			('groups:create', 'Create groups', 'groups', 'create', TRUE),
			('groups:read', 'View group information', 'groups', 'read', TRUE),
			('groups:update', 'Update groups', 'groups', 'update', TRUE),
			('groups:delete', 'Delete groups', 'groups', 'delete', TRUE),
			('groups:list', 'List groups', 'groups', 'list', TRUE),
			('groups:manage', 'Full groups management', 'groups', 'manage', TRUE),
			('groups:assign', 'Assign students/teachers to groups', 'groups', 'assign', TRUE),

			-- Activities permissions
			('activities:create', 'Create activities', 'activities', 'create', TRUE),
			('activities:read', 'View activity information', 'activities', 'read', TRUE),
			('activities:update', 'Update activities', 'activities', 'update', TRUE),
			('activities:delete', 'Delete activities', 'activities', 'delete', TRUE),
			('activities:list', 'List activities', 'activities', 'list', TRUE),
			('activities:manage', 'Full activities management', 'activities', 'manage', TRUE),
			('activities:enroll', 'Enroll students in activities', 'activities', 'enroll', TRUE),
			('activities:assign', 'Assign supervisors to activities', 'activities', 'assign', TRUE),

			-- Rooms permissions
			('rooms:read', 'View room information', 'rooms', 'read', TRUE),
			('rooms:list', 'List rooms', 'rooms', 'list', TRUE),

			-- Schedules permissions
			('schedules:read', 'View schedule information', 'schedules', 'read', TRUE),
			('schedules:list', 'List schedules', 'schedules', 'list', TRUE),

			-- Feedback permissions
			('feedback:read', 'View feedback', 'feedback', 'read', TRUE),
			('feedback:list', 'List feedback entries', 'feedback', 'list', TRUE)
		ON CONFLICT (name) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("error inserting permissions: %w", err)
	}

	// Step 3: Assign permissions to TEACHER role
	_, err = tx.ExecContext(ctx, `
		WITH teacher_role AS (
			SELECT id FROM auth.roles WHERE name = 'teacher' LIMIT 1
		),
		teacher_permissions AS (
			SELECT id FROM auth.permissions
			WHERE name IN (
				-- Users
				'users:read', 'users:update', 'users:list',
				-- Groups
				'groups:create', 'groups:read', 'groups:update', 'groups:delete',
				'groups:list', 'groups:manage', 'groups:assign',
				-- Activities
				'activities:create', 'activities:read', 'activities:update', 'activities:delete',
				'activities:list', 'activities:manage', 'activities:enroll', 'activities:assign',
				-- Visits (CRITICAL for check-in/check-out)
				'visits:create', 'visits:read', 'visits:update', 'visits:delete', 'visits:list',
				-- Rooms
				'rooms:read', 'rooms:list',
				-- Substitutions
				'substitutions:create', 'substitutions:read', 'substitutions:update',
				'substitutions:delete', 'substitutions:list', 'substitutions:manage',
				-- Schedules
				'schedules:read', 'schedules:list',
				-- Feedback
				'feedback:read', 'feedback:list'
			)
		)
		INSERT INTO auth.role_permissions (role_id, permission_id)
		SELECT tr.id, tp.id
		FROM teacher_role tr
		CROSS JOIN teacher_permissions tp
		WHERE tr.id IS NOT NULL
		ON CONFLICT (role_id, permission_id) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("error assigning permissions to teacher role: %w", err)
	}

	// Step 4: Assign permissions to STAFF role
	_, err = tx.ExecContext(ctx, `
		WITH staff_role AS (
			SELECT id FROM auth.roles WHERE name = 'staff' LIMIT 1
		),
		staff_permissions AS (
			SELECT id FROM auth.permissions
			WHERE name IN (
				-- Users (read only)
				'users:read', 'users:list',
				-- Groups (read only)
				'groups:read', 'groups:list',
				-- Activities (read only)
				'activities:read', 'activities:list',
				-- Visits (CRITICAL - staff handles check-ins/check-outs)
				'visits:create', 'visits:read', 'visits:update', 'visits:delete', 'visits:list',
				-- Rooms (read only)
				'rooms:read', 'rooms:list',
				-- Substitutions (read only)
				'substitutions:read', 'substitutions:list',
				-- Schedules (read only)
				'schedules:read', 'schedules:list'
			)
		)
		INSERT INTO auth.role_permissions (role_id, permission_id)
		SELECT sr.id, sp.id
		FROM staff_role sr
		CROSS JOIN staff_permissions sp
		WHERE sr.id IS NOT NULL
		ON CONFLICT (role_id, permission_id) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("error assigning permissions to staff role: %w", err)
	}

	// Step 5: Assign permissions to GUARDIAN role
	_, err = tx.ExecContext(ctx, `
		WITH guardian_role AS (
			SELECT id FROM auth.roles WHERE name = 'guardian' LIMIT 1
		),
		guardian_permissions AS (
			SELECT id FROM auth.permissions
			WHERE name IN (
				-- Minimal access - further restricted by policies
				'users:read',
				'groups:read',
				'visits:read'
			)
		)
		INSERT INTO auth.role_permissions (role_id, permission_id)
		SELECT gr.id, gp.id
		FROM guardian_role gr
		CROSS JOIN guardian_permissions gp
		WHERE gr.id IS NOT NULL
		ON CONFLICT (role_id, permission_id) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("error assigning permissions to guardian role: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	fmt.Println("Migration 1.7.4: Successfully added domain roles with permissions")
	return nil
}

// removeDomainRolesWithPermissions rolls back this migration
// Note: Only removes role_permissions assignments, keeps roles and permissions
// to avoid breaking existing account-role assignments
func removeDomainRolesWithPermissions(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.7.4: Removing domain role permission assignments...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && err == nil {
			err = rbErr
		}
	}()

	// Remove role_permissions for domain roles (safe - doesn't delete roles themselves)
	_, err = tx.ExecContext(ctx, `
		DELETE FROM auth.role_permissions
		WHERE role_id IN (
			SELECT id FROM auth.roles
			WHERE name IN ('teacher', 'staff', 'guardian')
		)
	`)
	if err != nil {
		return fmt.Errorf("error removing role_permissions: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	fmt.Println("Migration 1.7.4: Successfully removed domain role permission assignments")
	return nil
}
