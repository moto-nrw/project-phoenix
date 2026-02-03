package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	grantSuggestionsAllRolesVersion     = "1.9.3"
	grantSuggestionsAllRolesDescription = "Grant suggestions permissions to all system roles"
)

func init() {
	MigrationRegistry[grantSuggestionsAllRolesVersion] = &Migration{
		Version:     grantSuggestionsAllRolesVersion,
		Description: grantSuggestionsAllRolesDescription,
		DependsOn:   []string{"1.9.2", "1.7.4"}, // Depends on suggestions permissions + domain roles
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return grantSuggestionsToAllRoles(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return revokeExtraSuggestionsGrants(ctx, db)
		},
	)
}

func grantSuggestionsToAllRoles(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.9.3: Granting suggestions permissions to all system roles...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Grant suggestions permissions to ALL roles (admin and teacher already have them,
	// ON CONFLICT DO NOTHING makes this idempotent)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO auth.role_permissions (role_id, permission_id)
		SELECT r.id, p.id
		FROM auth.roles r
		CROSS JOIN auth.permissions p
		WHERE p.resource = 'suggestions'
		ON CONFLICT (role_id, permission_id) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("error granting suggestions permissions to all roles: %w", err)
	}

	fmt.Println("Migration 1.9.3: Successfully granted suggestions permissions to all roles")
	return tx.Commit()
}

func revokeExtraSuggestionsGrants(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.9.3: Revoking extra suggestions grants...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Remove suggestions permissions from roles that were NOT granted in 1.9.2
	// (i.e., keep admin and teacher, remove the rest)
	_, err = tx.ExecContext(ctx, `
		DELETE FROM auth.role_permissions
		WHERE permission_id IN (
			SELECT id FROM auth.permissions WHERE resource = 'suggestions'
		)
		AND role_id NOT IN (
			SELECT id FROM auth.roles WHERE name IN ('admin', 'teacher')
		)
	`)
	if err != nil {
		return fmt.Errorf("error revoking extra suggestions grants: %w", err)
	}

	fmt.Println("Migration 1.9.3: Successfully revoked extra suggestions grants")
	return tx.Commit()
}
