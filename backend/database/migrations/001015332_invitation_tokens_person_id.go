package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	invitationTokensPersonIDVersion     = "1.15.332"
	invitationTokensPersonIDDescription = "Link staff invitations to an already imported person so accepting reuses the record (#2600)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     invitationTokensPersonIDVersion,
		Description: invitationTokensPersonIDDescription,
		DependsOn:   []string{invitationTokensCaregiverEnabledVersion},
	})

	Migrations.MustRegister(invitationTokensPersonIDUp, invitationTokensPersonIDDown)
}

func invitationTokensPersonIDUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.332: Adding person_id to auth.invitation_tokens...")

	_, err := db.NewRaw(`
		ALTER TABLE auth.invitation_tokens
			ADD COLUMN IF NOT EXISTS person_id BIGINT
				REFERENCES users.persons(id) ON DELETE SET NULL;
		COMMENT ON COLUMN auth.invitation_tokens.person_id IS
			'Person the invitee becomes when accepting (set by the staff import); NULL = a person is created on acceptance';
		CREATE INDEX IF NOT EXISTS idx_invitation_tokens_person_id
			ON auth.invitation_tokens (person_id)
			WHERE person_id IS NOT NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("add person_id to auth.invitation_tokens: %w", err)
	}
	return nil
}

func invitationTokensPersonIDDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.332: Removing person_id from auth.invitation_tokens...")

	_, err := db.NewRaw(`
		DROP INDEX IF EXISTS auth.idx_invitation_tokens_person_id;
		ALTER TABLE auth.invitation_tokens
			DROP COLUMN IF EXISTS person_id;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("remove person_id from auth.invitation_tokens: %w", err)
	}
	return nil
}
