package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	AssignTeacherRoleVersion     = "1.7.5"
	AssignTeacherRoleDescription = "Assign teacher role to existing staff accounts"
)

func init() {
	MigrationRegistry[AssignTeacherRoleVersion] = &Migration{
		Version:     AssignTeacherRoleVersion,
		Description: AssignTeacherRoleDescription,
		DependsOn:   []string{"1.7.4"}, // Depends on domain roles creation
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return assignTeacherRoleToStaff(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return removeTeacherRoleFromStaff(ctx, db)
		},
	)
}

// assignTeacherRoleToStaff assigns the teacher role to all existing staff accounts
// This ensures existing deployments have staff visible in the group transfer dropdown
func assignTeacherRoleToStaff(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.7.5: Assigning teacher role to existing staff accounts...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && err == nil {
			err = rbErr
		}
	}()

	// Assign teacher role to all accounts linked to staff members
	// Path: auth.accounts ← users.persons ← users.staff
	result, err := tx.ExecContext(ctx, `
		INSERT INTO auth.account_roles (account_id, role_id, created_at, updated_at)
		SELECT DISTINCT
			p.account_id,
			r.id,
			NOW(),
			NOW()
		FROM users.persons p
		INNER JOIN users.staff s ON s.person_id = p.id
		INNER JOIN auth.roles r ON r.name = 'teacher'
		WHERE p.account_id IS NOT NULL
		ON CONFLICT (account_id, role_id) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("error assigning teacher role to staff: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	fmt.Printf("  ✓ Assigned teacher role to %d staff accounts\n", rowsAffected)

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	fmt.Println("Migration 1.7.5: Successfully assigned teacher role to staff accounts")
	return nil
}

// removeTeacherRoleFromStaff rolls back this migration by removing teacher role from staff
// Note: Only removes roles that were assigned by this migration (staff-linked accounts)
func removeTeacherRoleFromStaff(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.7.5: Removing teacher role from staff accounts...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && err == nil {
			err = rbErr
		}
	}()

	// Remove teacher role from staff-linked accounts
	result, err := tx.ExecContext(ctx, `
		DELETE FROM auth.account_roles ar
		USING users.persons p, users.staff s, auth.roles r
		WHERE ar.account_id = p.account_id
		  AND s.person_id = p.id
		  AND ar.role_id = r.id
		  AND r.name = 'teacher'
	`)
	if err != nil {
		return fmt.Errorf("error removing teacher role from staff: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	fmt.Printf("  ✓ Removed teacher role from %d staff accounts\n", rowsAffected)

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	fmt.Println("Migration 1.7.5: Successfully removed teacher role from staff accounts")
	return nil
}
