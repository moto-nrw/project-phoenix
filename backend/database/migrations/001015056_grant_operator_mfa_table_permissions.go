package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	grantOperatorMFATablePermissionsVersion     = "1.15.56"
	grantOperatorMFATablePermissionsDescription = "Grant phoenix_auth CRUD permissions on platform.operator_mfa_* tables (missed in migration 1.15.54)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     grantOperatorMFATablePermissionsVersion,
		Description: grantOperatorMFATablePermissionsDescription,
		DependsOn:   []string{"1.15.55"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.56: Granting phoenix_auth access to platform.operator_mfa_* tables...")

			// platform.operator_mfa_* tables are accessed from
			// /operator/auth/mfa/* handlers, which run on the phoenix_auth
			// connection (outside any tenant transaction). Mirrors the
			// pattern of platform.operator_email_change_tokens in 1.15.26.
			tables := []string{
				"operator_mfa_credentials",
				"operator_mfa_email_challenges",
				"operator_mfa_recovery_codes",
				"operator_mfa_trusted_devices",
			}
			var stmts []string
			for _, t := range tables {
				stmts = append(stmts,
					fmt.Sprintf(`GRANT SELECT, INSERT, UPDATE, DELETE ON platform.%s TO phoenix_auth;`, t),
					fmt.Sprintf(`GRANT USAGE, SELECT ON SEQUENCE platform.%s_id_seq TO phoenix_auth;`, t),
				)
			}
			for _, stmt := range stmts {
				if _, err := db.ExecContext(ctx, stmt); err != nil {
					return fmt.Errorf("grant failed (%s): %w", stmt, err)
				}
			}

			fmt.Println("Migration 1.15.56: Done")
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.56: Revoking phoenix_auth access to platform.operator_mfa_* tables...")

			tables := []string{
				"operator_mfa_credentials",
				"operator_mfa_email_challenges",
				"operator_mfa_recovery_codes",
				"operator_mfa_trusted_devices",
			}
			var stmts []string
			for _, t := range tables {
				stmts = append(stmts,
					fmt.Sprintf(`REVOKE SELECT, INSERT, UPDATE, DELETE ON platform.%s FROM phoenix_auth;`, t),
					fmt.Sprintf(`REVOKE USAGE, SELECT ON SEQUENCE platform.%s_id_seq FROM phoenix_auth;`, t),
				)
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
