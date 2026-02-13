package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	createTenantRolesVersion     = "1.14.1"
	createTenantRolesDescription = "Create PostgreSQL roles for multi-tenancy (phoenix_auth, phoenix_tenant, phoenix_admin)"
)

func init() {
	MigrationRegistry[createTenantRolesVersion] = &Migration{
		Version:     createTenantRolesVersion,
		Description: createTenantRolesDescription,
		DependsOn:   []string{"1.13.1"}, // Depends on platform schema/tables
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createTenantRoles(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return rollbackTenantRoles(ctx, db)
		},
	)
}

func createTenantRoles(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.14.1: Creating PostgreSQL roles for multi-tenancy...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Create connection role: LOGIN + NOINHERIT (zero privileges by default)
	_, err = tx.ExecContext(ctx, `
		DO $$ BEGIN
			IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'phoenix_auth') THEN
				CREATE ROLE phoenix_auth LOGIN NOINHERIT PASSWORD 'phoenix_auth_dev';
			END IF;
		END $$;
	`)
	if err != nil {
		return fmt.Errorf("error creating phoenix_auth role: %w", err)
	}

	// Create tenant role: subject to RLS, CRUD on tenant-scoped schemas
	_, err = tx.ExecContext(ctx, `
		DO $$ BEGIN
			IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'phoenix_tenant') THEN
				CREATE ROLE phoenix_tenant NOLOGIN;
			END IF;
		END $$;
	`)
	if err != nil {
		return fmt.Errorf("error creating phoenix_tenant role: %w", err)
	}

	// Create admin role: BYPASSRLS for operator routes, migrations, seeds, cross-tenant
	_, err = tx.ExecContext(ctx, `
		DO $$ BEGIN
			IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'phoenix_admin') THEN
				CREATE ROLE phoenix_admin NOLOGIN BYPASSRLS;
			END IF;
		END $$;
	`)
	if err != nil {
		return fmt.Errorf("error creating phoenix_admin role: %w", err)
	}

	// phoenix_auth can switch to either role via SET ROLE
	_, err = tx.ExecContext(ctx, `
		GRANT phoenix_tenant TO phoenix_auth;
		GRANT phoenix_admin TO phoenix_auth;
	`)
	if err != nil {
		return fmt.Errorf("error granting roles to phoenix_auth: %w", err)
	}

	// phoenix_tenant: CRUD on all tenant-scoped schemas + SELECT on platform.schools
	_, err = tx.ExecContext(ctx, `
		GRANT USAGE ON SCHEMA auth, users, education, facilities, activities, active,
			schedule, iot, feedback, config, suggestions, audit TO phoenix_tenant;

		GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA
			auth, users, education, facilities, activities, active,
			schedule, iot, feedback, config, suggestions, audit TO phoenix_tenant;

		GRANT SELECT ON platform.schools TO phoenix_tenant;

		GRANT USAGE ON ALL SEQUENCES IN SCHEMA
			auth, users, education, facilities, activities, active,
			schedule, iot, feedback, config, suggestions, audit TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error granting permissions to phoenix_tenant: %w", err)
	}

	// Default privileges for future tables/sequences (phoenix_tenant)
	_, err = tx.ExecContext(ctx, `
		ALTER DEFAULT PRIVILEGES IN SCHEMA
			auth, users, education, facilities, activities, active,
			schedule, iot, feedback, config, suggestions, audit
			GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO phoenix_tenant;

		ALTER DEFAULT PRIVILEGES IN SCHEMA
			auth, users, education, facilities, activities, active,
			schedule, iot, feedback, config, suggestions, audit
			GRANT USAGE ON SEQUENCES TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error setting default privileges for phoenix_tenant: %w", err)
	}

	// phoenix_admin: ALL on everything including platform
	_, err = tx.ExecContext(ctx, `
		GRANT USAGE ON SCHEMA auth, users, education, facilities, activities, active,
			schedule, iot, feedback, config, suggestions, audit, platform TO phoenix_admin;

		GRANT ALL ON ALL TABLES IN SCHEMA
			auth, users, education, facilities, activities, active,
			schedule, iot, feedback, config, suggestions, audit, platform TO phoenix_admin;

		GRANT ALL ON ALL SEQUENCES IN SCHEMA
			auth, users, education, facilities, activities, active,
			schedule, iot, feedback, config, suggestions, audit, platform TO phoenix_admin;
	`)
	if err != nil {
		return fmt.Errorf("error granting permissions to phoenix_admin: %w", err)
	}

	// Default privileges for future tables/sequences (phoenix_admin)
	_, err = tx.ExecContext(ctx, `
		ALTER DEFAULT PRIVILEGES IN SCHEMA
			auth, users, education, facilities, activities, active,
			schedule, iot, feedback, config, suggestions, audit, platform
			GRANT ALL ON TABLES TO phoenix_admin;

		ALTER DEFAULT PRIVILEGES IN SCHEMA
			auth, users, education, facilities, activities, active,
			schedule, iot, feedback, config, suggestions, audit, platform
			GRANT ALL ON SEQUENCES TO phoenix_admin;
	`)
	if err != nil {
		return fmt.Errorf("error setting default privileges for phoenix_admin: %w", err)
	}

	fmt.Println("Migration 1.14.1: Successfully created PostgreSQL roles for multi-tenancy")
	return tx.Commit()
}

func rollbackTenantRoles(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.14.1: Dropping tenant PostgreSQL roles...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Revoke default privileges first
	_, err = tx.ExecContext(ctx, `
		ALTER DEFAULT PRIVILEGES IN SCHEMA
			auth, users, education, facilities, activities, active,
			schedule, iot, feedback, config, suggestions, audit, platform
			REVOKE ALL ON TABLES FROM phoenix_admin;

		ALTER DEFAULT PRIVILEGES IN SCHEMA
			auth, users, education, facilities, activities, active,
			schedule, iot, feedback, config, suggestions, audit, platform
			REVOKE ALL ON SEQUENCES FROM phoenix_admin;

		ALTER DEFAULT PRIVILEGES IN SCHEMA
			auth, users, education, facilities, activities, active,
			schedule, iot, feedback, config, suggestions, audit
			REVOKE ALL ON TABLES FROM phoenix_tenant;

		ALTER DEFAULT PRIVILEGES IN SCHEMA
			auth, users, education, facilities, activities, active,
			schedule, iot, feedback, config, suggestions, audit
			REVOKE ALL ON SEQUENCES FROM phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error revoking default privileges: %w", err)
	}

	// Revoke all grants
	_, err = tx.ExecContext(ctx, `
		REVOKE ALL ON ALL TABLES IN SCHEMA
			auth, users, education, facilities, activities, active,
			schedule, iot, feedback, config, suggestions, audit, platform FROM phoenix_admin;
		REVOKE ALL ON ALL SEQUENCES IN SCHEMA
			auth, users, education, facilities, activities, active,
			schedule, iot, feedback, config, suggestions, audit, platform FROM phoenix_admin;
		REVOKE USAGE ON SCHEMA auth, users, education, facilities, activities, active,
			schedule, iot, feedback, config, suggestions, audit, platform FROM phoenix_admin;

		REVOKE ALL ON ALL TABLES IN SCHEMA
			auth, users, education, facilities, activities, active,
			schedule, iot, feedback, config, suggestions, audit FROM phoenix_tenant;
		REVOKE ALL ON ALL SEQUENCES IN SCHEMA
			auth, users, education, facilities, activities, active,
			schedule, iot, feedback, config, suggestions, audit FROM phoenix_tenant;
		REVOKE SELECT ON platform.schools FROM phoenix_tenant;
		REVOKE USAGE ON SCHEMA auth, users, education, facilities, activities, active,
			schedule, iot, feedback, config, suggestions, audit FROM phoenix_tenant;

		REVOKE phoenix_tenant FROM phoenix_auth;
		REVOKE phoenix_admin FROM phoenix_auth;
	`)
	if err != nil {
		return fmt.Errorf("error revoking grants: %w", err)
	}

	// Drop roles
	_, err = tx.ExecContext(ctx, `
		DROP ROLE IF EXISTS phoenix_auth;
		DROP ROLE IF EXISTS phoenix_tenant;
		DROP ROLE IF EXISTS phoenix_admin;
	`)
	if err != nil {
		return fmt.Errorf("error dropping roles: %w", err)
	}

	fmt.Println("Migration 1.14.1: Successfully rolled back tenant PostgreSQL roles")
	return tx.Commit()
}
