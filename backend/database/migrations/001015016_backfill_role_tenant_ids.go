package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	backfillRoleTenantIDsVersion     = "1.15.16"
	backfillRoleTenantIDsDescription = "Delete orphaned non-system roles with NULL tenant_id"
)

func init() {
	MigrationRegistry[backfillRoleTenantIDsVersion] = &Migration{
		Version:     backfillRoleTenantIDsVersion,
		Description: backfillRoleTenantIDsDescription,
		DependsOn:   []string{"1.15.15"},
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return backfillRoleTenantIDs(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			// No rollback — deleted rows cannot be restored
			return nil
		},
	)
}

// backfillRoleTenantIDs removes non-system roles that have NULL tenant_id.
//
// Before tenant scoping was added to RoleRepository, custom roles were created
// without a tenant_id. These orphaned rows are now visible to ALL tenants via
// the "tenant_id = ? OR tenant_id IS NULL" read queries — a cross-tenant leak.
//
// System roles (is_system = true) intentionally have NULL tenant_id and are
// shared across all tenants, so they are left untouched.
func backfillRoleTenantIDs(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.16: Cleaning up orphaned non-system roles with NULL tenant_id...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// First, clean up any references in account_roles and role_permissions
	// for orphaned roles that are about to be deleted.
	_, err = tx.ExecContext(ctx, `
		DELETE FROM auth.account_roles
		WHERE role_id IN (
			SELECT id FROM auth.roles
			WHERE tenant_id IS NULL AND is_system = false
		)
	`)
	if err != nil {
		return fmt.Errorf("error cleaning up account_roles for orphaned roles: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		DELETE FROM auth.role_permissions
		WHERE role_id IN (
			SELECT id FROM auth.roles
			WHERE tenant_id IS NULL AND is_system = false
		)
	`)
	if err != nil {
		return fmt.Errorf("error cleaning up role_permissions for orphaned roles: %w", err)
	}

	// Delete the orphaned roles themselves
	result, err := tx.ExecContext(ctx, `
		DELETE FROM auth.roles
		WHERE tenant_id IS NULL AND is_system = false
	`)
	if err != nil {
		return fmt.Errorf("error deleting orphaned non-system roles: %w", err)
	}

	affected, _ := result.RowsAffected()
	fmt.Printf("Migration 1.15.16: Deleted %d orphaned non-system role(s)\n", affected)

	return tx.Commit()
}
