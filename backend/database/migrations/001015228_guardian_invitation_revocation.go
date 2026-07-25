package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	guardianInvitationRevocationVersion     = "1.15.228"
	guardianInvitationRevocationDescription = "Add revoked_at/revoked_reason to auth.guardian_invitations so a token minted for a mistyped address can be invalidated (#1937)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     guardianInvitationRevocationVersion,
		Description: guardianInvitationRevocationDescription,
		DependsOn:   []string{AuthGuardianInvitationsVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return guardianInvitationRevocationUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return guardianInvitationRevocationDown(ctx, db)
		},
	)
}

func guardianInvitationRevocationUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.228: Adding revocation columns to auth.guardian_invitations...")

	// A separate column rather than back-dating expires_at: "zurückgezogen"
	// and "abgelaufen" need different UI text, and rewriting expires_at would
	// destroy the record of what the token's lifetime actually was.
	if _, err := db.NewRaw(`
		ALTER TABLE auth.guardian_invitations
			ADD COLUMN IF NOT EXISTS revoked_at     TIMESTAMPTZ,
			ADD COLUMN IF NOT EXISTS revoked_reason TEXT;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed adding revocation columns: %w", err)
	}

	if _, err := db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_guardian_invitations_open
			ON auth.guardian_invitations (tenant_id, guardian_profile_id)
			WHERE accepted_at IS NULL AND revoked_at IS NULL;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating open-invitation index: %w", err)
	}

	return nil
}

func guardianInvitationRevocationDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.228: Dropping revocation columns...")
	if _, err := db.NewRaw(`
		DROP INDEX IF EXISTS auth.idx_guardian_invitations_open;
		ALTER TABLE auth.guardian_invitations
			DROP COLUMN IF EXISTS revoked_at,
			DROP COLUMN IF EXISTS revoked_reason;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping revocation columns: %w", err)
	}
	return nil
}
