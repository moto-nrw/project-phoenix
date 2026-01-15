package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/uptrace/bun"
)

const (
	RestrictSubstitutionPermissionsVersion     = "1.7.1"
	RestrictSubstitutionPermissionsDescription = "Restrict substitution write permissions to admin role only"
)

func init() {
	MigrationRegistry[RestrictSubstitutionPermissionsVersion] = &Migration{
		Version:     RestrictSubstitutionPermissionsVersion,
		Description: RestrictSubstitutionPermissionsDescription,
		DependsOn: []string{
			"1.6.8", // Depends on migration that added substitution permissions to teachers
		},
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return restrictSubstitutionPermissionsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return restrictSubstitutionPermissionsDown(ctx, db)
		},
	)
}

func restrictSubstitutionPermissionsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.6.21: Restricting substitution write permissions to admin role...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
				logrus.Warnf("Failed to rollback transaction: %v", rbErr)
			}
		}
	}()

	// Remove substitution write permissions from teacher role
	// Teachers should only use /api/groups/{id}/transfer for same-day transfers
	// Admin substitutions (/api/substitutions) are for multi-day coverage
	_, err = tx.ExecContext(ctx, `
		DELETE FROM auth.role_permissions
		WHERE role_id = (SELECT id FROM auth.roles WHERE name = 'teacher')
		AND permission_id IN (
			SELECT id FROM auth.permissions
			WHERE name IN (
				'substitutions:create',
				'substitutions:update',
				'substitutions:delete'
			)
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to remove substitution write permissions from teacher role: %w", err)
	}

	// Remove substitution write permissions from staff role (if exists)
	_, err = tx.ExecContext(ctx, `
		DELETE FROM auth.role_permissions
		WHERE role_id = (SELECT id FROM auth.roles WHERE name = 'staff')
		AND permission_id IN (
			SELECT id FROM auth.permissions
			WHERE name IN (
				'substitutions:create',
				'substitutions:update',
				'substitutions:delete'
			)
		);
	`)
	if err != nil {
		// Staff role might not exist, which is okay
		logrus.Warnf("Note: Could not remove permissions from staff role (role might not exist): %v", err)
	}

	// Teachers keep substitutions:read for viewing their own transfers
	// via /api/groups/{id}/substitutions (which uses groups:read permission)

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	committed = true

	logrus.Info("Successfully restricted substitution write permissions to admin role")
	return nil
}

func restrictSubstitutionPermissionsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.6.21: Restoring substitution permissions to teacher role...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
				logrus.Warnf("Failed to rollback transaction: %v", rbErr)
			}
		}
	}()

	// Restore substitution permissions to teacher role
	_, err = tx.ExecContext(ctx, `
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
			'substitutions:create',
			'substitutions:update',
			'substitutions:delete'
		)
		ON CONFLICT (role_id, permission_id) DO NOTHING;
	`)
	if err != nil {
		return fmt.Errorf("failed to restore substitution permissions to teacher role: %w", err)
	}

	// Restore substitution permissions to staff role (if exists)
	_, err = tx.ExecContext(ctx, `
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
			'substitutions:create',
			'substitutions:update',
			'substitutions:delete'
		)
		ON CONFLICT (role_id, permission_id) DO NOTHING;
	`)
	if err != nil {
		// Staff role might not exist, which is okay
		logrus.Warnf("Note: Could not restore permissions to staff role (role might not exist): %v", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	committed = true

	logrus.Info("Successfully restored substitution permissions to teacher role")
	return nil
}
