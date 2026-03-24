package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	backfillRoleTenantIDsVersion     = "1.15.17"
	backfillRoleTenantIDsDescription = "Backfill tenant_id on custom roles from existing references, delete truly orphaned ones"
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
			// Rollback is not meaningful — backfilled tenant_ids are correct,
			// and deleted orphans had no references to restore.
			return nil
		},
	)
}

// backfillRoleTenantIDs repairs non-system roles that have NULL tenant_id.
//
// Before tenant scoping was added to RoleRepository, custom roles were created
// without a tenant_id. These rows are now visible to ALL tenants via the
// "tenant_id = ? OR tenant_id IS NULL" read queries — a cross-tenant leak.
//
// Strategy:
//  1. Derive tenant_id from account_roles (most reliable — active assignments)
//  2. Derive tenant_id from invitation_tokens (covers roles only used in invites)
//  3. Expire pending invitation_tokens that reference still-unresolvable roles
//  4. Clean up account_roles, role_permissions for truly orphaned roles
//  5. Delete only truly orphaned roles (no tenant derivable from any source)
//
// System roles (is_system = true) intentionally have NULL tenant_id and are
// shared across all tenants, so they are left untouched.
func backfillRoleTenantIDs(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.17: Backfilling tenant_id on non-system roles with NULL tenant_id...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Step 1: Backfill tenant_id from account_roles.
	// account_roles already has tenant_id (set by migration 1.14.2).
	// Use the most common tenant_id across assignments for each role.
	result, err := tx.ExecContext(ctx, `
		UPDATE auth.roles r
		SET tenant_id = sub.tenant_id
		FROM (
			SELECT DISTINCT ON (ar.role_id) ar.role_id, ar.tenant_id
			FROM auth.account_roles ar
			WHERE ar.tenant_id IS NOT NULL
			  AND ar.role_id IN (
			      SELECT id FROM auth.roles
			      WHERE tenant_id IS NULL AND is_system = false
			  )
			GROUP BY ar.role_id, ar.tenant_id
			ORDER BY ar.role_id, COUNT(*) DESC
		) sub
		WHERE r.id = sub.role_id
	`)
	if err != nil {
		return fmt.Errorf("error backfilling role tenant_id from account_roles: %w", err)
	}
	backfilledFromAR, _ := result.RowsAffected()
	fmt.Printf("Migration 1.15.17: Backfilled %d role(s) from account_roles\n", backfilledFromAR)

	// Step 2: Backfill remaining NULL roles from invitation_tokens.
	// invitation_tokens also has tenant_id (set by migration 1.14.2).
	result, err = tx.ExecContext(ctx, `
		UPDATE auth.roles r
		SET tenant_id = sub.tenant_id
		FROM (
			SELECT DISTINCT ON (it.role_id) it.role_id, it.tenant_id
			FROM auth.invitation_tokens it
			WHERE it.tenant_id IS NOT NULL
			  AND it.role_id IN (
			      SELECT id FROM auth.roles
			      WHERE tenant_id IS NULL AND is_system = false
			  )
			ORDER BY it.role_id, it.created_at DESC
		) sub
		WHERE r.id = sub.role_id
	`)
	if err != nil {
		return fmt.Errorf("error backfilling role tenant_id from invitation_tokens: %w", err)
	}
	backfilledFromIT, _ := result.RowsAffected()
	fmt.Printf("Migration 1.15.17: Backfilled %d role(s) from invitation_tokens\n", backfilledFromIT)

	// Step 3: Mark pending invitation_tokens as expired for truly orphaned roles.
	// These roles have no derivable tenant — the invitations are unusable anyway.
	// Must do this BEFORE deleting the roles due to ON DELETE RESTRICT.
	result, err = tx.ExecContext(ctx, `
		UPDATE auth.invitation_tokens
		SET used_at = NOW()
		WHERE used_at IS NULL
		  AND role_id IN (
		      SELECT id FROM auth.roles
		      WHERE tenant_id IS NULL AND is_system = false
		  )
	`)
	if err != nil {
		return fmt.Errorf("error expiring invitation_tokens for orphaned roles: %w", err)
	}
	expiredInvites, _ := result.RowsAffected()
	if expiredInvites > 0 {
		fmt.Printf("Migration 1.15.17: Expired %d invitation(s) referencing orphaned roles\n", expiredInvites)
	}

	// Step 4: Clean up account_roles and role_permissions for truly orphaned roles.
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

	// Step 5: Delete truly orphaned roles (still NULL tenant_id after backfill).
	result, err = tx.ExecContext(ctx, `
		DELETE FROM auth.roles
		WHERE tenant_id IS NULL AND is_system = false
	`)
	if err != nil {
		return fmt.Errorf("error deleting orphaned non-system roles: %w", err)
	}
	deleted, _ := result.RowsAffected()
	fmt.Printf("Migration 1.15.17: Deleted %d truly orphaned role(s) with no derivable tenant\n", deleted)

	return tx.Commit()
}
