package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	activityCategoryManagePermissionVersion     = "1.15.258"
	activityCategoryManagePermissionDescription = "Add activities:manage_categories permission for school-admin category Stammdaten (issue #2131)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     activityCategoryManagePermissionVersion,
		Description: activityCategoryManagePermissionDescription,
		DependsOn:   []string{activityCategoryArchivalVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addActivityCategoryManagePermission(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return removeActivityCategoryManagePermission(ctx, db)
		},
	)
}

// addActivityCategoryManagePermission introduces activities:manage_categories
// and grants it to the admin role only. The existing activities:* permissions
// cannot serve as the gate: migration 1.9.4 granted activities:manage (and
// create/update/delete) to the plain `user` role, so every Betreuer holds
// them. Category Stammdaten are school-wide configuration and must stay with
// the OGS-Leitung, hence a dedicated admin-only permission.
func addActivityCategoryManagePermission(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.258: Adding activities:manage_categories permission...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	if _, err = tx.ExecContext(ctx, `
		INSERT INTO auth.permissions (name, description, resource, action)
		VALUES
			('activities:manage_categories', 'Manage activity categories (school Stammdaten)', 'activities', 'manage_categories')
		ON CONFLICT (name) DO NOTHING
	`); err != nil {
		return fmt.Errorf("error inserting activities:manage_categories: %w", err)
	}

	if _, err = tx.ExecContext(ctx, `
		INSERT INTO auth.role_permissions (role_id, permission_id)
		SELECT r.id, p.id
		FROM auth.roles r
		CROSS JOIN auth.permissions p
		WHERE p.name = 'activities:manage_categories'
		  AND r.name = 'admin'
		ON CONFLICT (role_id, permission_id) DO NOTHING
	`); err != nil {
		return fmt.Errorf("error granting activities:manage_categories to admin: %w", err)
	}

	fmt.Println("Migration 1.15.258: Successfully granted activities:manage_categories to admin role")
	return tx.Commit()
}

func removeActivityCategoryManagePermission(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.258: Removing activities:manage_categories...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	if _, err = tx.ExecContext(ctx, `
		DELETE FROM auth.role_permissions
		WHERE permission_id IN (
			SELECT id FROM auth.permissions WHERE name = 'activities:manage_categories'
		)
	`); err != nil {
		return fmt.Errorf("error removing role permissions: %w", err)
	}

	if _, err = tx.ExecContext(ctx, `
		DELETE FROM auth.permissions WHERE name = 'activities:manage_categories'
	`); err != nil {
		return fmt.Errorf("error removing permission: %w", err)
	}

	fmt.Println("Migration 1.15.258: Successfully rolled back")
	return tx.Commit()
}
