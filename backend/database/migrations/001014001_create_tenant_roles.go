package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

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
	// Password MUST be set via environment variable — no hardcoded fallback.
	authPassword := os.Getenv("PHOENIX_AUTH_PASSWORD")
	if authPassword == "" {
		return fmt.Errorf("PHOENIX_AUTH_PASSWORD environment variable is required but not set. " +
			"Add it to your dev.env (local) or .env (Docker) file. " +
			"See dev.env.example for the expected format")
	}
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		DO $$ BEGIN
			IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'phoenix_auth') THEN
				CREATE ROLE phoenix_auth LOGIN NOINHERIT PASSWORD %s;
			END IF;
		END $$;
	`, quoteLiteral(authPassword)))
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

	// phoenix_auth: direct permissions for pre-auth operations (login, tenant resolve).
	// These endpoints run WITHOUT WithAdminTx/WithTenantTx because there is no
	// authenticated context yet. phoenix_auth is NOINHERIT, so it needs explicit grants.
	// search_path includes platform so BUN's Relation() joins resolve correctly.
	_, err = tx.ExecContext(ctx, `
		-- Tenant resolution (public, pre-auth)
		GRANT USAGE ON SCHEMA platform TO phoenix_auth;
		GRANT SELECT ON platform.schools, platform.organizations TO phoenix_auth;
		ALTER ROLE phoenix_auth SET search_path TO public, platform;

		-- Platform announcements (no tx wrapper, runs as phoenix_auth directly)
		GRANT SELECT ON platform.announcements, platform.announcement_views TO phoenix_auth;
		GRANT INSERT, UPDATE ON platform.announcement_views TO phoenix_auth;
		GRANT USAGE ON ALL SEQUENCES IN SCHEMA platform TO phoenix_auth;

		-- Login flow: credential validation, role/permission loading, token management
		GRANT USAGE ON SCHEMA auth, users, audit TO phoenix_auth;
		GRANT SELECT ON auth.accounts, auth.account_tenants, auth.account_roles,
			auth.roles, auth.permissions, auth.account_permissions,
			auth.role_permissions TO phoenix_auth;
		GRANT UPDATE ON auth.accounts TO phoenix_auth;
		GRANT SELECT, INSERT, DELETE ON auth.tokens TO phoenix_auth;
		GRANT USAGE ON ALL SEQUENCES IN SCHEMA auth TO phoenix_auth;
		GRANT SELECT ON users.persons TO phoenix_auth;
		GRANT INSERT ON audit.auth_events TO phoenix_auth;
		GRANT USAGE ON ALL SEQUENCES IN SCHEMA audit TO phoenix_auth;
	`)
	if err != nil {
		return fmt.Errorf("error granting pre-auth permissions to phoenix_auth: %w", err)
	}

	// phoenix_tenant: CRUD on all tenant-scoped schemas + SELECT on platform.schools
	// USAGE on platform schema required for SELECT on platform.schools to work
	_, err = tx.ExecContext(ctx, `
		GRANT USAGE ON SCHEMA auth, users, education, facilities, activities, active,
			schedule, iot, feedback, config, suggestions, audit, platform TO phoenix_tenant;

		GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA
			auth, users, education, facilities, activities, active,
			schedule, iot, feedback, config, suggestions, audit TO phoenix_tenant;

		GRANT SELECT ON platform.schools, platform.announcements TO phoenix_tenant;
		GRANT SELECT, INSERT, UPDATE ON platform.announcement_views TO phoenix_tenant;

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

	// phoenix_admin: ALL on everything including platform and meta
	_, err = tx.ExecContext(ctx, `
		GRANT USAGE ON SCHEMA auth, users, education, facilities, activities, active,
			schedule, iot, feedback, config, suggestions, audit, platform, meta TO phoenix_admin;

		GRANT ALL ON ALL TABLES IN SCHEMA
			auth, users, education, facilities, activities, active,
			schedule, iot, feedback, config, suggestions, audit, platform, meta TO phoenix_admin;

		GRANT ALL ON ALL SEQUENCES IN SCHEMA
			auth, users, education, facilities, activities, active,
			schedule, iot, feedback, config, suggestions, audit, platform, meta TO phoenix_admin;
	`)
	if err != nil {
		return fmt.Errorf("error granting permissions to phoenix_admin: %w", err)
	}

	// Default privileges for future tables/sequences (phoenix_admin)
	_, err = tx.ExecContext(ctx, `
		ALTER DEFAULT PRIVILEGES IN SCHEMA
			auth, users, education, facilities, activities, active,
			schedule, iot, feedback, config, suggestions, audit, platform, meta
			GRANT ALL ON TABLES TO phoenix_admin;

		ALTER DEFAULT PRIVILEGES IN SCHEMA
			auth, users, education, facilities, activities, active,
			schedule, iot, feedback, config, suggestions, audit, platform, meta
			GRANT ALL ON SEQUENCES TO phoenix_admin;
	`)
	if err != nil {
		return fmt.Errorf("error setting default privileges for phoenix_admin: %w", err)
	}

	fmt.Println("Migration 1.14.1: Successfully created PostgreSQL roles for multi-tenancy")
	return tx.Commit()
}

// quoteLiteral safely quotes a string for use as a PostgreSQL literal,
// preventing SQL injection by escaping single quotes.
func quoteLiteral(s string) string {
	// Replace single quotes with doubled single quotes (PostgreSQL escaping)
	escaped := ""
	for _, c := range s {
		if c == '\'' {
			escaped += "''"
		} else {
			escaped += string(c)
		}
	}
	return "'" + escaped + "'"
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
			schedule, iot, feedback, config, suggestions, audit, platform, meta
			REVOKE ALL ON TABLES FROM phoenix_admin;

		ALTER DEFAULT PRIVILEGES IN SCHEMA
			auth, users, education, facilities, activities, active,
			schedule, iot, feedback, config, suggestions, audit, platform, meta
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
			schedule, iot, feedback, config, suggestions, audit, platform, meta FROM phoenix_admin;
		REVOKE ALL ON ALL SEQUENCES IN SCHEMA
			auth, users, education, facilities, activities, active,
			schedule, iot, feedback, config, suggestions, audit, platform, meta FROM phoenix_admin;
		REVOKE USAGE ON SCHEMA auth, users, education, facilities, activities, active,
			schedule, iot, feedback, config, suggestions, audit, platform, meta FROM phoenix_admin;

		REVOKE ALL ON ALL TABLES IN SCHEMA
			auth, users, education, facilities, activities, active,
			schedule, iot, feedback, config, suggestions, audit FROM phoenix_tenant;
		REVOKE ALL ON ALL SEQUENCES IN SCHEMA
			auth, users, education, facilities, activities, active,
			schedule, iot, feedback, config, suggestions, audit FROM phoenix_tenant;
		REVOKE SELECT ON platform.schools, platform.announcements FROM phoenix_tenant;
		REVOKE SELECT, INSERT, UPDATE ON platform.announcement_views FROM phoenix_tenant;
		REVOKE USAGE ON SCHEMA auth, users, education, facilities, activities, active,
			schedule, iot, feedback, config, suggestions, audit, platform FROM phoenix_tenant;

		ALTER ROLE phoenix_auth RESET search_path;
		REVOKE ALL ON ALL TABLES IN SCHEMA auth, audit FROM phoenix_auth;
		REVOKE ALL ON ALL SEQUENCES IN SCHEMA auth, audit FROM phoenix_auth;
		REVOKE SELECT ON users.persons FROM phoenix_auth;
		REVOKE SELECT ON platform.schools, platform.organizations,
			platform.announcements, platform.announcement_views FROM phoenix_auth;
		REVOKE INSERT, UPDATE ON platform.announcement_views FROM phoenix_auth;
		REVOKE USAGE ON ALL SEQUENCES IN SCHEMA platform FROM phoenix_auth;
		REVOKE USAGE ON SCHEMA platform, auth, users, audit FROM phoenix_auth;

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
