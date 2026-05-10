package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	grantMFATablePermissionsVersion     = "1.15.55"
	grantMFATablePermissionsDescription = "Grant phoenix_auth + phoenix_tenant CRUD permissions on auth.mfa_* tables (missed in migrations 1.15.49–52)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     grantMFATablePermissionsVersion,
		Description: grantMFATablePermissionsDescription,
		DependsOn:   []string{"1.15.54"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.55: Granting phoenix_tenant access to auth.mfa_* tables...")

			// Two roles need access to auth.mfa_* tables:
			//   - phoenix_auth  — the base connection role used by
			//     /auth/mfa/* endpoints (StartChallenge, enroll, verify, …).
			//     These run outside TenantTxMiddleware, so no SET LOCAL ROLE
			//     is issued and queries hit the default phoenix_auth grants.
			//     This mirrors the password_reset_tokens pattern in 1.15.13.
			//   - phoenix_tenant — used inside TenantTxMiddleware. While the
			//     MFA service today doesn't query inside a tenant tx, granting
			//     to both roles keeps the surface consistent and lets future
			//     callers (e.g. tenant-scoped read endpoints) work without
			//     another migration.
			//
			// The auth.mfa_* tables have no RLS — the service enforces account
			// scoping via account_id filters — so plain CRUD grants are fine.
			roles := []string{"phoenix_auth", "phoenix_tenant"}
			tables := []string{
				// mfa_credentials: enroll (INSERT), check (SELECT),
				// update last_used_at (UPDATE), disable (DELETE).
				"mfa_credentials",
				// mfa_email_challenges: rate-limit count (SELECT), persist
				// new challenge (INSERT), mark consumed (UPDATE), cleanup
				// (DELETE).
				"mfa_email_challenges",
				// mfa_recovery_codes: bulk-insert on enroll, mark used,
				// regenerate (DELETE + INSERT).
				"mfa_recovery_codes",
				// mfa_trusted_devices: issue, verify, revoke, cleanup.
				"mfa_trusted_devices",
			}
			var stmts []string
			for _, role := range roles {
				for _, t := range tables {
					stmts = append(stmts,
						fmt.Sprintf(`GRANT SELECT, INSERT, UPDATE, DELETE ON auth.%s TO %s;`, t, role),
						fmt.Sprintf(`GRANT USAGE, SELECT ON SEQUENCE auth.%s_id_seq TO %s;`, t, role),
					)
				}
			}
			for _, stmt := range stmts {
				if _, err := db.ExecContext(ctx, stmt); err != nil {
					return fmt.Errorf("grant failed (%s): %w", stmt, err)
				}
			}

			fmt.Println("Migration 1.15.55: Done")
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.55: Revoking phoenix_auth + phoenix_tenant access to auth.mfa_* tables...")

			roles := []string{"phoenix_auth", "phoenix_tenant"}
			tables := []string{
				"mfa_credentials",
				"mfa_email_challenges",
				"mfa_recovery_codes",
				"mfa_trusted_devices",
			}
			var stmts []string
			for _, role := range roles {
				for _, t := range tables {
					stmts = append(stmts,
						fmt.Sprintf(`REVOKE SELECT, INSERT, UPDATE, DELETE ON auth.%s FROM %s;`, t, role),
						fmt.Sprintf(`REVOKE USAGE, SELECT ON SEQUENCE auth.%s_id_seq FROM %s;`, t, role),
					)
				}
			}
			for _, stmt := range stmts {
				if _, err := db.ExecContext(ctx, stmt); err != nil {
					return fmt.Errorf("revoke failed (%s): %w", stmt, err)
				}
			}
			return nil
		},
	)
}
