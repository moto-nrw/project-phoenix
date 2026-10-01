package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	parentMessageStaffMarkedUnreadVersion     = "1.15.422"
	parentMessageStaffMarkedUnreadDescription = "Let staff mark a parent conversation unread for the whole team (#3654)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentMessageStaffMarkedUnreadVersion,
		Description: parentMessageStaffMarkedUnreadDescription,
		DependsOn:   []string{parentMessageTeamHandledVersion},
	})
	Migrations.MustRegister(parentMessageStaffMarkedUnreadUp, parentMessageStaffMarkedUnreadDown)
}

// parentMessageStaffMarkedUnreadUp adds the team-wide "als ungelesen markiert"
// state to the conversation. It is separate from the read cursors and the team
// handled boundary on purpose: both of those only move forward and feed the
// parent-facing "Von der OGS gelesen" receipt, which marking must not change.
// The table already carries tenant RLS, so the new columns inherit it.
func parentMessageStaffMarkedUnreadUp(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		ALTER TABLE users.parent_message_threads
			ADD COLUMN IF NOT EXISTS staff_marked_unread_at TIMESTAMPTZ,
			ADD COLUMN IF NOT EXISTS staff_marked_unread_by_account_id BIGINT;

		ALTER TABLE users.parent_message_threads
			DROP CONSTRAINT IF EXISTS chk_parent_message_threads_staff_marked_unread,
			ADD CONSTRAINT chk_parent_message_threads_staff_marked_unread CHECK (
				(staff_marked_unread_at IS NULL) = (staff_marked_unread_by_account_id IS NULL)
			) NOT VALID;

		ALTER TABLE users.parent_message_threads
			VALIDATE CONSTRAINT chk_parent_message_threads_staff_marked_unread;
	`)
	if err != nil {
		return fmt.Errorf("add parent-message staff marked-unread state: %w", err)
	}
	return nil
}

func parentMessageStaffMarkedUnreadDown(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		ALTER TABLE users.parent_message_threads
			DROP CONSTRAINT IF EXISTS chk_parent_message_threads_staff_marked_unread,
			DROP COLUMN IF EXISTS staff_marked_unread_by_account_id,
			DROP COLUMN IF EXISTS staff_marked_unread_at;
	`)
	if err != nil {
		return fmt.Errorf("remove parent-message staff marked-unread state: %w", err)
	}
	return nil
}
