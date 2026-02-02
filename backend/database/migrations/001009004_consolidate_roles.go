package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	consolidateRolesVersion     = "1.9.4"
	consolidateRolesDescription = "Consolidate roles: expand user permissions, remove teacher/staff roles"
)

func init() {
	MigrationRegistry[consolidateRolesVersion] = &Migration{
		Version:     consolidateRolesVersion,
		Description: consolidateRolesDescription,
		DependsOn:   []string{"1.9.3", "1.7.4"}, // Depends on latest migration + domain roles
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return consolidateRolesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return consolidateRolesDown(ctx, db)
		},
	)
}

func consolidateRolesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.9.4: Consolidating roles — expanding user permissions, removing teacher/staff roles...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Step 1: Add 22 permissions to user role (ON CONFLICT DO NOTHING for idempotency)
	_, err = tx.ExecContext(ctx, `
		WITH user_role AS (
			SELECT id FROM auth.roles WHERE name = 'user' LIMIT 1
		),
		new_permissions AS (
			SELECT id FROM auth.permissions
			WHERE name IN (
				'groups:read', 'groups:update', 'groups:list',
				'activities:update', 'activities:delete', 'activities:list',
				'activities:manage', 'activities:enroll', 'activities:assign',
				'visits:create', 'visits:read', 'visits:update',
				'visits:delete', 'visits:list',
				'users:list',
				'rooms:list',
				'schedules:read', 'schedules:list',
				'feedback:read', 'feedback:list',
				'substitutions:read'
			)
		)
		INSERT INTO auth.role_permissions (role_id, permission_id)
		SELECT ur.id, np.id
		FROM user_role ur
		CROSS JOIN new_permissions np
		WHERE ur.id IS NOT NULL
		ON CONFLICT (role_id, permission_id) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("error adding permissions to user role: %w", err)
	}

	// Step 2: Reassign any accounts on teacher/staff roles → user role
	_, err = tx.ExecContext(ctx, `
		UPDATE auth.account_roles
		SET role_id = (SELECT id FROM auth.roles WHERE name = 'user' LIMIT 1),
		    updated_at = NOW()
		WHERE role_id IN (
			SELECT id FROM auth.roles WHERE name IN ('teacher', 'staff')
		)
		AND account_id NOT IN (
			SELECT account_id FROM auth.account_roles
			WHERE role_id = (SELECT id FROM auth.roles WHERE name = 'user' LIMIT 1)
		)
	`)
	if err != nil {
		return fmt.Errorf("error reassigning accounts from teacher/staff to user: %w", err)
	}

	// Remove duplicate role assignments (accounts that already had user role)
	_, err = tx.ExecContext(ctx, `
		DELETE FROM auth.account_roles
		WHERE role_id IN (
			SELECT id FROM auth.roles WHERE name IN ('teacher', 'staff')
		)
	`)
	if err != nil {
		return fmt.Errorf("error cleaning up remaining teacher/staff account_roles: %w", err)
	}

	// Step 3: Delete role_permissions for teacher/staff
	_, err = tx.ExecContext(ctx, `
		DELETE FROM auth.role_permissions
		WHERE role_id IN (
			SELECT id FROM auth.roles WHERE name IN ('teacher', 'staff')
		)
	`)
	if err != nil {
		return fmt.Errorf("error deleting teacher/staff role_permissions: %w", err)
	}

	// Step 4: Delete teacher/staff roles
	_, err = tx.ExecContext(ctx, `
		DELETE FROM auth.roles WHERE name IN ('teacher', 'staff')
	`)
	if err != nil {
		return fmt.Errorf("error deleting teacher/staff roles: %w", err)
	}

	fmt.Println("Migration 1.9.4: Successfully consolidated roles")
	return tx.Commit()
}

func consolidateRolesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.10.1: Restoring teacher/staff roles...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Re-create teacher and staff roles
	_, err = tx.ExecContext(ctx, `
		INSERT INTO auth.roles (name, description, is_system, created_at, updated_at)
		VALUES
			('teacher', 'Teacher role with group management permissions', true, NOW(), NOW()),
			('staff', 'Staff role with visit management permissions', true, NOW(), NOW())
		ON CONFLICT (name) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("error re-creating teacher/staff roles: %w", err)
	}

	// Re-assign teacher permissions (matching original 1.7.4 migration)
	_, err = tx.ExecContext(ctx, `
		WITH teacher_role AS (
			SELECT id FROM auth.roles WHERE name = 'teacher' LIMIT 1
		),
		teacher_permissions AS (
			SELECT id FROM auth.permissions
			WHERE name IN (
				'users:read', 'users:list',
				'groups:read', 'groups:update', 'groups:list',
				'activities:create', 'activities:read', 'activities:update',
				'activities:delete', 'activities:list', 'activities:manage',
				'activities:enroll', 'activities:assign',
				'visits:create', 'visits:read', 'visits:update',
				'visits:delete', 'visits:list', 'visits:manage',
				'rooms:read', 'rooms:list',
				'schedules:read', 'schedules:list',
				'feedback:create', 'feedback:read', 'feedback:list',
				'config:read',
				'substitutions:read', 'substitutions:create',
				'substitutions:update', 'substitutions:delete', 'substitutions:manage'
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
		return fmt.Errorf("error re-assigning teacher permissions: %w", err)
	}

	// Re-assign staff permissions
	_, err = tx.ExecContext(ctx, `
		WITH staff_role AS (
			SELECT id FROM auth.roles WHERE name = 'staff' LIMIT 1
		),
		staff_permissions AS (
			SELECT id FROM auth.permissions
			WHERE name IN (
				'users:read', 'users:list',
				'groups:read', 'groups:list',
				'activities:read', 'activities:list',
				'visits:create', 'visits:read', 'visits:update',
				'visits:delete', 'visits:list', 'visits:manage',
				'rooms:read', 'rooms:list',
				'schedules:read', 'schedules:list',
				'feedback:read', 'feedback:list'
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
		return fmt.Errorf("error re-assigning staff permissions: %w", err)
	}

	// Remove the extra permissions that were added to user role
	_, err = tx.ExecContext(ctx, `
		DELETE FROM auth.role_permissions
		WHERE role_id = (SELECT id FROM auth.roles WHERE name = 'user' LIMIT 1)
		AND permission_id IN (
			SELECT id FROM auth.permissions
			WHERE name IN (
				'groups:read', 'groups:update', 'groups:list',
				'activities:update', 'activities:delete', 'activities:list',
				'activities:manage', 'activities:enroll', 'activities:assign',
				'visits:create', 'visits:read', 'visits:update',
				'visits:delete', 'visits:list',
				'users:list',
				'rooms:list',
				'schedules:read', 'schedules:list',
				'feedback:read', 'feedback:list',
				'substitutions:read'
			)
		)
	`)
	if err != nil {
		return fmt.Errorf("error removing extra user permissions: %w", err)
	}

	fmt.Println("Migration 1.9.4: Successfully restored teacher/staff roles")
	return tx.Commit()
}
