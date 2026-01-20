package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	CreateBypassRoleVersion     = "1.8.4"
	CreateBypassRoleDescription = "Create phoenix_admin role with BYPASSRLS for maintenance operations"
)

func init() {
	MigrationRegistry[CreateBypassRoleVersion] = &Migration{
		Version:     CreateBypassRoleVersion,
		Description: CreateBypassRoleDescription,
		DependsOn:   []string{"1.8.3"}, // After RLS policies are enabled
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createBypassRole(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropBypassRole(ctx, db)
		},
	)
}

// createBypassRole creates the phoenix_admin role with BYPASSRLS capability.
// This role is used for:
// - Running migrations that need access to all tenant data
// - Maintenance operations (data cleanup, auditing)
// - Cross-tenant reporting (if needed)
// - Emergency debugging
//
// Usage:
//
//	SET ROLE phoenix_admin;
//	-- queries now bypass RLS, can see all data
//	RESET ROLE;
//
// SECURITY NOTE: This role has NOLOGIN, meaning it cannot be used to connect
// directly. It must be assumed via SET ROLE from an already-authenticated session.
func createBypassRole(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.8.4: Creating phoenix_admin role with BYPASSRLS...")

	// Create role if it doesn't exist
	// BYPASSRLS: Can bypass Row-Level Security policies
	// NOLOGIN: Cannot be used to directly connect (must use SET ROLE)
	_, err := db.ExecContext(ctx, `
		DO $$
		BEGIN
			IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'phoenix_admin') THEN
				CREATE ROLE phoenix_admin WITH
					BYPASSRLS
					NOLOGIN;

				COMMENT ON ROLE phoenix_admin IS
					'Admin role that bypasses RLS. Use for migrations, maintenance, and cross-tenant operations.
					 Usage: SET ROLE phoenix_admin; ... RESET ROLE;
					 This role has NOLOGIN - must be assumed via SET ROLE from authenticated session.';
			END IF;
		END
		$$;
	`)
	if err != nil {
		return fmt.Errorf("error creating phoenix_admin role: %w", err)
	}

	fmt.Println("  Created phoenix_admin role")

	// Grant necessary permissions to the role
	// Note: We grant to existing schemas. Some may not exist in all environments.
	schemas := []string{
		"users", "education", "facilities", "iot", "active",
		"activities", "tenant", "auth", "schedule", "feedback", "config", "audit",
	}

	for _, schema := range schemas {
		// Grant usage on schema
		usageSQL := fmt.Sprintf(`GRANT USAGE ON SCHEMA %s TO phoenix_admin`, schema)
		_, err = db.ExecContext(ctx, usageSQL)
		if err != nil {
			// Non-fatal - schema might not exist
			fmt.Printf("  Warning: Could not grant USAGE on schema %s: %v\n", schema, err)
			continue
		}

		// Grant all privileges on existing tables in schema
		tablesSQL := fmt.Sprintf(`GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA %s TO phoenix_admin`, schema)
		_, err = db.ExecContext(ctx, tablesSQL)
		if err != nil {
			fmt.Printf("  Warning: Could not grant privileges on tables in %s: %v\n", schema, err)
		}

		// Grant all privileges on future tables in schema (for new migrations)
		defaultSQL := fmt.Sprintf(`
			ALTER DEFAULT PRIVILEGES IN SCHEMA %s
			GRANT ALL PRIVILEGES ON TABLES TO phoenix_admin
		`, schema)
		_, err = db.ExecContext(ctx, defaultSQL)
		if err != nil {
			fmt.Printf("  Warning: Could not set default privileges for %s: %v\n", schema, err)
		}
	}

	fmt.Println("Migration 1.8.4: Successfully created phoenix_admin role")
	fmt.Println("  Usage: SET ROLE phoenix_admin; <query>; RESET ROLE;")
	return nil
}

// dropBypassRole removes the phoenix_admin role.
// This revokes all privileges and then drops the role.
func dropBypassRole(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.8.4: Dropping phoenix_admin role...")

	// Check if role exists first
	var exists bool
	err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT FROM pg_roles WHERE rolname = 'phoenix_admin')`).Scan(&exists)
	if err != nil {
		return fmt.Errorf("error checking if phoenix_admin exists: %w", err)
	}

	if !exists {
		fmt.Println("  phoenix_admin role does not exist, nothing to drop")
		return nil
	}

	// Revoke privileges from all schemas (ignore errors for non-existent schemas)
	schemas := []string{
		"users", "education", "facilities", "iot", "active",
		"activities", "tenant", "auth", "schedule", "feedback", "config", "audit",
	}

	for _, schema := range schemas {
		revokeSQL := fmt.Sprintf(`REVOKE ALL PRIVILEGES ON ALL TABLES IN SCHEMA %s FROM phoenix_admin`, schema)
		_, _ = db.ExecContext(ctx, revokeSQL) // Ignore errors
	}

	// Drop the role
	_, err = db.ExecContext(ctx, `DROP ROLE IF EXISTS phoenix_admin`)
	if err != nil {
		return fmt.Errorf("error dropping phoenix_admin role: %w", err)
	}

	fmt.Println("Migration 1.8.4: Successfully dropped phoenix_admin role")
	return nil
}
