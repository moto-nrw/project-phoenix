package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	grantOperatorPlatformPermissionsVersion     = "1.15.10"
	grantOperatorPlatformPermissionsDescription = "Grant phoenix_auth permissions for operator platform features (audit log, announcements CRUD, suggestions)"
)

func init() {
	MigrationRegistry[grantOperatorPlatformPermissionsVersion] = &Migration{
		Version:     grantOperatorPlatformPermissionsVersion,
		Description: grantOperatorPlatformPermissionsDescription,
		DependsOn:   []string{"1.15.9"},
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.10: Granting operator platform permissions to phoenix_auth...")

			// Operator services (announcements, suggestions, auth) run directly as
			// phoenix_auth without WithAdminTx. They need explicit grants because
			// phoenix_auth is NOINHERIT.
			_, err := db.ExecContext(ctx, `
				-- Audit log: operator login and action logging
				GRANT SELECT, INSERT ON platform.operator_audit_log TO phoenix_auth;

				-- Announcements: full CRUD (SELECT already granted in 1.14.1)
				GRANT INSERT, UPDATE, DELETE ON platform.announcements TO phoenix_auth;

				-- Suggestions schema: operator comment/status management and unread counts
				GRANT USAGE ON SCHEMA suggestions TO phoenix_auth;
				GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA suggestions TO phoenix_auth;
				GRANT USAGE ON ALL SEQUENCES IN SCHEMA suggestions TO phoenix_auth;

				-- Default privileges for future suggestions tables
				ALTER DEFAULT PRIVILEGES IN SCHEMA suggestions
					GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO phoenix_auth;
				ALTER DEFAULT PRIVILEGES IN SCHEMA suggestions
					GRANT USAGE ON SEQUENCES TO phoenix_auth;
			`)
			if err != nil {
				return fmt.Errorf("error granting operator platform permissions to phoenix_auth: %w", err)
			}

			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.10: Revoking operator platform permissions from phoenix_auth...")

			_, err := db.ExecContext(ctx, `
				ALTER DEFAULT PRIVILEGES IN SCHEMA suggestions
					REVOKE SELECT, INSERT, UPDATE, DELETE ON TABLES FROM phoenix_auth;
				ALTER DEFAULT PRIVILEGES IN SCHEMA suggestions
					REVOKE USAGE ON SEQUENCES FROM phoenix_auth;

				REVOKE SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA suggestions FROM phoenix_auth;
				REVOKE USAGE ON ALL SEQUENCES IN SCHEMA suggestions FROM phoenix_auth;
				REVOKE USAGE ON SCHEMA suggestions FROM phoenix_auth;

				REVOKE INSERT, UPDATE, DELETE ON platform.announcements FROM phoenix_auth;
				REVOKE SELECT, INSERT ON platform.operator_audit_log FROM phoenix_auth;
			`)
			if err != nil {
				return fmt.Errorf("error revoking operator platform permissions from phoenix_auth: %w", err)
			}

			return nil
		},
	)
}
