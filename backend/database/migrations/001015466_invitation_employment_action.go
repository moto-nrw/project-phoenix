package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     "1.15.466",
		Description: "Store the school’s employment choice on invitations (#3404)",
		DependsOn:   []string{schoolContentRetentionVersion},
	})
	Migrations.MustRegister(invitationEmploymentActionUp, invitationEmploymentActionDown)
}

func invitationEmploymentActionUp(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `ALTER TABLE auth.invitation_tokens
  ADD COLUMN employment_action TEXT NOT NULL DEFAULT 'resume'
  CHECK (employment_action IN ('resume', 'new'))`)
	if err != nil {
		return fmt.Errorf("add invitation employment action: %w", err)
	}
	return nil
}

func invitationEmploymentActionDown(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `ALTER TABLE auth.invitation_tokens DROP COLUMN employment_action`)
	if err != nil {
		return fmt.Errorf("drop invitation employment action: %w", err)
	}
	return nil
}
