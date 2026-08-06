package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	guardianInvitationRoleUpgradeVersion     = "1.15.270"
	guardianInvitationRoleUpgradeDescription = "Track pending guardian invitations that upgrade an existing restricted contact link on approval (#2172)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     guardianInvitationRoleUpgradeVersion,
		Description: guardianInvitationRoleUpgradeDescription,
		DependsOn:   []string{deviceTransfersVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return guardianInvitationRoleUpgradeUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return guardianInvitationRoleUpgradeDown(ctx, db)
		},
	)
}

func guardianInvitationRoleUpgradeUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.270: Tracking guardian-invitation role upgrades...")

	if _, err := db.NewRaw(`
		ALTER TABLE auth.guardian_invitations
			ADD COLUMN IF NOT EXISTS role_upgrade BOOLEAN NOT NULL DEFAULT FALSE;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("error adding role_upgrade: %w", err)
	}
	return nil
}

func guardianInvitationRoleUpgradeDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.270: dropping role_upgrade marker...")

	if _, err := db.NewRaw(`
		ALTER TABLE auth.guardian_invitations
			DROP COLUMN IF EXISTS role_upgrade;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("error dropping role_upgrade: %w", err)
	}
	return nil
}
