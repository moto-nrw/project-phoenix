package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	parentMessageTeamHandledVersion     = "1.15.287"
	parentMessageTeamHandledDescription = "Track guardian messages covered by a staff reply (#2246)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentMessageTeamHandledVersion,
		Description: parentMessageTeamHandledDescription,
		DependsOn:   []string{parentMessagingVersion},
	})
	Migrations.MustRegister(parentMessageTeamHandledUp, parentMessageTeamHandledDown)
}

func parentMessageTeamHandledUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.287: Adding the parent-message team handled cursor...")
	_, err := db.ExecContext(ctx, `
		ALTER TABLE users.parent_message_threads
			ADD COLUMN IF NOT EXISTS staff_handled_up_to_at TIMESTAMPTZ,
			ADD COLUMN IF NOT EXISTS staff_handled_up_to_message_id BIGINT;

		ALTER TABLE users.parent_message_threads
			DROP CONSTRAINT IF EXISTS chk_parent_message_threads_staff_handled_cursor,
			ADD CONSTRAINT chk_parent_message_threads_staff_handled_cursor CHECK (
				(staff_handled_up_to_at IS NULL) = (staff_handled_up_to_message_id IS NULL)
			) NOT VALID;

		ALTER TABLE users.parent_message_threads
			VALIDATE CONSTRAINT chk_parent_message_threads_staff_handled_cursor;
	`)
	if err != nil {
		return fmt.Errorf("add parent-message team handled cursor: %w", err)
	}
	return nil
}

func parentMessageTeamHandledDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.287: Removing the parent-message team handled cursor...")
	_, err := db.ExecContext(ctx, `
		ALTER TABLE users.parent_message_threads
			DROP CONSTRAINT IF EXISTS chk_parent_message_threads_staff_handled_cursor,
			DROP COLUMN IF EXISTS staff_handled_up_to_message_id,
			DROP COLUMN IF EXISTS staff_handled_up_to_at;
	`)
	if err != nil {
		return fmt.Errorf("remove parent-message team handled cursor: %w", err)
	}
	return nil
}
