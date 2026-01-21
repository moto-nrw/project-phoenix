package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	CreateOgsRolePermissionsVersion     = "1.8.7"
	CreateOgsRolePermissionsDescription = "Create config tables for per-OGS permission customization"
)

func init() {
	MigrationRegistry[CreateOgsRolePermissionsVersion] = &Migration{
		Version:     CreateOgsRolePermissionsVersion,
		Description: CreateOgsRolePermissionsDescription,
		DependsOn:   []string{"1.8.6"}, // After betterauth_user_id
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createOgsRolePermissions(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropOgsRolePermissions(ctx, db)
		},
	)
}

// createOgsRolePermissions creates tables for per-OGS permission customization.
//
// This enables organizations to customize role permissions while staying within
// GDPR boundaries. For example, an OGS might want to:
// - Grant additional permissions to supervisors (e.g., schedule:create)
// - Revoke certain permissions from ogsAdmin (e.g., staff:delete)
//
// GDPR-CRITICAL RESTRICTIONS:
// - location:read can NEVER be granted to bueroAdmin or traegerAdmin roles
// - These restrictions are enforced in application code, not database constraints
//
// The permission resolution order is:
// 1. Check staff_permission_overrides for individual exceptions
// 2. Check ogs_role_permissions for OGS-specific role config
// 3. Fall back to default role permissions (auth/tenant/roles.go)
func createOgsRolePermissions(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.8.7: Creating OGS role permissions tables...")

	// Create table for per-OGS role permission customization
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS config.ogs_role_permissions (
			id BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
			ogs_id TEXT NOT NULL,
			role_name VARCHAR(50) NOT NULL,
			permission_key VARCHAR(100) NOT NULL,
			granted BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(ogs_id, role_name, permission_key)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating ogs_role_permissions table: %w", err)
	}
	fmt.Println("  Created config.ogs_role_permissions table")

	// Add index for fast lookups
	_, err = db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_ogs_role_permissions_lookup
		ON config.ogs_role_permissions(ogs_id, role_name)
	`)
	if err != nil {
		return fmt.Errorf("error creating ogs_role_permissions index: %w", err)
	}
	fmt.Println("  Created index idx_ogs_role_permissions_lookup")

	// Add comment documenting the table purpose
	_, err = db.ExecContext(ctx, `
		COMMENT ON TABLE config.ogs_role_permissions IS
			'Per-OGS role permission customization. Allows organizations to grant/revoke
			 specific permissions for roles within their OGS. GDPR restriction: location:read
			 can never be granted to bueroAdmin or traegerAdmin roles (enforced in app code).'
	`)
	if err != nil {
		return fmt.Errorf("error adding table comment: %w", err)
	}

	// Create table for individual staff permission overrides
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS config.staff_permission_overrides (
			id BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
			staff_id BIGINT NOT NULL REFERENCES users.staff(id) ON DELETE CASCADE,
			ogs_id TEXT NOT NULL,
			permission_key VARCHAR(100) NOT NULL,
			granted BOOLEAN NOT NULL,
			reason TEXT,
			granted_by BIGINT REFERENCES users.staff(id),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			expires_at TIMESTAMPTZ,
			UNIQUE(staff_id, ogs_id, permission_key)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating staff_permission_overrides table: %w", err)
	}
	fmt.Println("  Created config.staff_permission_overrides table")

	// Add indexes for staff permission lookups
	_, err = db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_staff_permission_overrides_staff
		ON config.staff_permission_overrides(staff_id, ogs_id)
	`)
	if err != nil {
		return fmt.Errorf("error creating staff_permission_overrides index: %w", err)
	}
	fmt.Println("  Created index idx_staff_permission_overrides_staff")

	// Add index for expired permissions cleanup
	_, err = db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_staff_permission_overrides_expires
		ON config.staff_permission_overrides(expires_at)
		WHERE expires_at IS NOT NULL
	`)
	if err != nil {
		return fmt.Errorf("error creating staff_permission_overrides expires index: %w", err)
	}
	fmt.Println("  Created index idx_staff_permission_overrides_expires")

	// Add comment documenting the table purpose
	_, err = db.ExecContext(ctx, `
		COMMENT ON TABLE config.staff_permission_overrides IS
			'Individual staff permission overrides. Allows granting/revoking specific
			 permissions for individual staff members with an optional expiry date.
			 Takes precedence over ogs_role_permissions. Includes audit trail (reason, granted_by).'
	`)
	if err != nil {
		return fmt.Errorf("error adding staff_permission_overrides comment: %w", err)
	}

	fmt.Println("Migration 1.8.7: Successfully created OGS role permissions tables")
	return nil
}

// dropOgsRolePermissions removes the permission customization tables.
func dropOgsRolePermissions(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.8.7: Dropping OGS role permissions tables...")

	// Drop staff overrides first (has FK to staff)
	_, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS config.staff_permission_overrides CASCADE`)
	if err != nil {
		return fmt.Errorf("error dropping staff_permission_overrides table: %w", err)
	}
	fmt.Println("  Dropped config.staff_permission_overrides table")

	// Drop OGS role permissions
	_, err = db.ExecContext(ctx, `DROP TABLE IF EXISTS config.ogs_role_permissions CASCADE`)
	if err != nil {
		return fmt.Errorf("error dropping ogs_role_permissions table: %w", err)
	}
	fmt.Println("  Dropped config.ogs_role_permissions table")

	fmt.Println("Migration 1.8.7: Successfully dropped OGS role permissions tables")
	return nil
}
