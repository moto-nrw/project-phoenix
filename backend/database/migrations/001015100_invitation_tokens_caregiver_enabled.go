package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	invitationTokensCaregiverEnabledVersion     = "1.15.100"
	invitationTokensCaregiverEnabledDescription = "Add caregiver_enabled flag to auth.invitation_tokens for operator admin invitations with caregiver capability"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     invitationTokensCaregiverEnabledVersion,
		Description: invitationTokensCaregiverEnabledDescription,
		DependsOn:   []string{AddPositionToInvitationTokensVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.100: Adding caregiver_enabled to auth.invitation_tokens...")
			if _, err := db.NewRaw(`
				ALTER TABLE auth.invitation_tokens
					ADD COLUMN IF NOT EXISTS caregiver_enabled boolean NOT NULL DEFAULT false;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding caregiver_enabled to auth.invitation_tokens: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.100: Removing caregiver_enabled from auth.invitation_tokens...")
			if _, err := db.NewRaw(`
				ALTER TABLE auth.invitation_tokens
					DROP COLUMN IF EXISTS caregiver_enabled;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping caregiver_enabled from auth.invitation_tokens: %w", err)
			}
			return nil
		},
	)
}
