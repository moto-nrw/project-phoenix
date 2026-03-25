package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	fixPhoenixAuthTokenGrantsVersion     = "1.15.13"
	fixPhoenixAuthTokenGrantsDescription = "Grant missing permissions to phoenix_auth for tokens, password reset, and profiles"
)

func init() {
	MigrationRegistry[fixPhoenixAuthTokenGrantsVersion] = &Migration{
		Version:     fixPhoenixAuthTokenGrantsVersion,
		Description: fixPhoenixAuthTokenGrantsDescription,
		DependsOn:   []string{"1.15.12"},
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.13: Fixing phoenix_auth grants...")

			// Fix 1: auth.tokens — add UPDATE for SELECT ... FOR UPDATE (defense-in-depth).
			// Migration 1.14.1 granted SELECT, INSERT, DELETE but missed UPDATE.
			// The refresh flow uses WithAdminTx, but logout runs as phoenix_auth directly
			// and may need UPDATE in the future. Harmless to grant now.
			_, err := db.ExecContext(ctx, `GRANT UPDATE ON auth.tokens TO phoenix_auth;`)
			if err != nil {
				return fmt.Errorf("grant UPDATE on auth.tokens: %w", err)
			}

			// Fix 2: users.profiles — add SELECT (defense-in-depth).
			// Currently the /me/profile endpoint uses TenantTxMiddleware (phoenix_tenant),
			// but phoenix_auth should also be able to read profiles for auth flows.
			_, err = db.ExecContext(ctx, `GRANT SELECT ON users.profiles TO phoenix_auth;`)
			if err != nil {
				return fmt.Errorf("grant SELECT on users.profiles: %w", err)
			}

			// Fix 3: auth.password_reset_tokens — phoenix_auth has ZERO grants.
			// The password reset flow (InitiatePasswordReset) uses RunInTx which runs
			// as phoenix_auth directly (no WithAdminTx). It needs:
			//   - SELECT: FindValidByToken (before WithAdminTx in ResetPassword)
			//   - INSERT: Create new reset tokens
			//   - UPDATE: InvalidateTokensByAccountID, MarkAsUsed, UpdateDeliveryResult
			_, err = db.ExecContext(ctx, `GRANT SELECT, INSERT, UPDATE ON auth.password_reset_tokens TO phoenix_auth;`)
			if err != nil {
				return fmt.Errorf("grant on auth.password_reset_tokens: %w", err)
			}

			// Fix 4: auth.password_reset_rate_limits — phoenix_auth has ZERO grants.
			// checkPasswordResetRateLimit runs as phoenix_auth and needs:
			//   - SELECT: CheckRateLimit
			//   - INSERT: IncrementAttempts (upsert)
			//   - UPDATE: IncrementAttempts (upsert)
			_, err = db.ExecContext(ctx, `GRANT SELECT, INSERT, UPDATE ON auth.password_reset_rate_limits TO phoenix_auth;`)
			if err != nil {
				return fmt.Errorf("grant on auth.password_reset_rate_limits: %w", err)
			}

			fmt.Println("Migration 1.15.13: Done")
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.13: Revoking phoenix_auth grants...")

			_, err := db.ExecContext(ctx, `
				REVOKE UPDATE ON auth.tokens FROM phoenix_auth;
				REVOKE SELECT ON users.profiles FROM phoenix_auth;
				REVOKE SELECT, INSERT, UPDATE ON auth.password_reset_tokens FROM phoenix_auth;
				REVOKE SELECT, INSERT, UPDATE ON auth.password_reset_rate_limits FROM phoenix_auth;
			`)
			if err != nil {
				return fmt.Errorf("error revoking phoenix_auth grants: %w", err)
			}

			return nil
		},
	)
}
