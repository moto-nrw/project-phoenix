package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	fixPhoenixAuthTokenGrantsVersion     = "1.15.13"
	fixPhoenixAuthTokenGrantsDescription = "Grant missing UPDATE on auth.tokens and SELECT on users.profiles to phoenix_auth (fixes FOR UPDATE and profile lookup)"
)

func init() {
	MigrationRegistry[fixPhoenixAuthTokenGrantsVersion] = &Migration{
		Version:     fixPhoenixAuthTokenGrantsVersion,
		Description: fixPhoenixAuthTokenGrantsDescription,
		DependsOn:   []string{"1.15.12"},
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.13: Fixing phoenix_auth token and profile grants...")

			// Fix 1: auth.tokens needs UPDATE for SELECT ... FOR UPDATE in refresh flow.
			// Migration 1.14.1 granted SELECT, INSERT, DELETE but missed UPDATE.
			// The token refresh uses FindByTokenForUpdate which does FOR UPDATE row locking.
			_, err := db.ExecContext(ctx, `GRANT UPDATE ON auth.tokens TO phoenix_auth;`)
			if err != nil {
				return fmt.Errorf("error granting UPDATE on auth.tokens to phoenix_auth: %w", err)
			}

			// Fix 2: users.profiles needs SELECT for the /me/profile endpoint.
			// This endpoint runs outside TenantTxMiddleware (auth context only),
			// so it queries as phoenix_auth directly.
			_, err = db.ExecContext(ctx, `GRANT SELECT ON users.profiles TO phoenix_auth;`)
			if err != nil {
				return fmt.Errorf("error granting SELECT on users.profiles to phoenix_auth: %w", err)
			}

			fmt.Println("Migration 1.15.13: Done")
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.13: Revoking phoenix_auth token and profile grants...")

			_, err := db.ExecContext(ctx, `
				REVOKE UPDATE ON auth.tokens FROM phoenix_auth;
				REVOKE SELECT ON users.profiles FROM phoenix_auth;
			`)
			if err != nil {
				return fmt.Errorf("error revoking phoenix_auth token and profile grants: %w", err)
			}

			return nil
		},
	)
}
