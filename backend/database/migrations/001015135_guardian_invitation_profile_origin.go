package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	guardianInvitationProfileOriginVersion     = "1.15.135"
	guardianInvitationProfileOriginDescription = "Track guardian profiles created solely for a guardian invitation"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     guardianInvitationProfileOriginVersion,
		Description: guardianInvitationProfileOriginDescription,
		DependsOn:   []string{guardianInvitationsParentInitiatedVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return guardianInvitationProfileOriginUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return guardianInvitationProfileOriginDown(ctx, db)
		},
	)
}

func guardianInvitationProfileOriginUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.135: Tracking invite-created guardian profiles...")

	if _, err := db.NewRaw(`
		ALTER TABLE auth.guardian_invitations
			ADD COLUMN IF NOT EXISTS profile_created_for_invitation BOOLEAN NOT NULL DEFAULT FALSE;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("error adding profile_created_for_invitation: %w", err)
	}
	return nil
}

func guardianInvitationProfileOriginDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.135: dropping invite-created profile marker...")

	if _, err := db.NewRaw(`
		ALTER TABLE auth.guardian_invitations
			DROP COLUMN IF EXISTS profile_created_for_invitation;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("error dropping profile_created_for_invitation: %w", err)
	}
	return nil
}
