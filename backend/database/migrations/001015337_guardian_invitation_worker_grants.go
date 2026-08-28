package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	guardianInvitationWorkerGrantsVersion     = "1.15.337"
	guardianInvitationWorkerGrantsDescription = "Grant phoenix_auth the guardian invitation tables needed by the email worker"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     guardianInvitationWorkerGrantsVersion,
		Description: guardianInvitationWorkerGrantsDescription,
		DependsOn: []string{
			"1.6.16",  // auth.guardian_invitations
			"1.3.6",   // users.students_guardians
			"1.14.1",  // phoenix_auth
			"1.15.58", // platform.email_outbox
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.337: Granting guardian invitation worker permissions to phoenix_auth...")
			if _, err := db.ExecContext(ctx, `
				GRANT SELECT ON users.students_guardians TO phoenix_auth;
				GRANT SELECT, UPDATE ON auth.guardian_invitations TO phoenix_auth;
			`); err != nil {
				return fmt.Errorf("grant guardian invitation worker permissions to phoenix_auth: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.337: Revoking guardian invitation worker permissions from phoenix_auth...")
			if _, err := db.ExecContext(ctx, `
				REVOKE SELECT ON users.students_guardians FROM phoenix_auth;
				REVOKE SELECT, UPDATE ON auth.guardian_invitations FROM phoenix_auth;
			`); err != nil {
				return fmt.Errorf("revoke guardian invitation worker permissions from phoenix_auth: %w", err)
			}
			return nil
		},
	)
}
