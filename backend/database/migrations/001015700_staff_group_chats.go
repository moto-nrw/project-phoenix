package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	staffGroupChatsVersion     = "1.15.700"
	staffGroupChatsDescription = "Team-Chat: Gruppenchats mit Namen, Ersteller und Verlauf ab Beitritt (#4776)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffGroupChatsVersion,
		Description: staffGroupChatsDescription,
		DependsOn: []string{
			"1.15.693",            // the last migration before this one
			staffMessagingVersion, // the tables this extends
		},
	})
	Migrations.MustRegister(staffGroupChatsUp, staffGroupChatsDown)
}

// staffGroupChatsUp opens the staff conversation tables for group chats
// (#4776). The 1:1 chat (#2598) already modelled participants as rows and the
// thread identity as an opaque key, so a group is a new `kind`, not a new
// table:
//
//   - name holds the group's display name; a direct chat has none, a group
//     must have one (chk_staff_message_threads_group_name).
//   - created_by_account_id records who started the group: that person and
//     the school leadership (when a member) may manage members and the name.
//     SET NULL on account deletion keeps the group; then only members of the
//     leadership manage it.
//   - history_from on a participant row is the instant the member joined a
//     group. Messages before it stay invisible to them and never count as
//     unread. NULL means the whole history (every direct chat participant).
func staffGroupChatsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `
		ALTER TABLE users.staff_message_threads
			ADD COLUMN IF NOT EXISTS name TEXT,
			ADD COLUMN IF NOT EXISTS created_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL;

		ALTER TABLE users.staff_message_threads
			DROP CONSTRAINT IF EXISTS chk_staff_message_threads_kind;
		ALTER TABLE users.staff_message_threads
			ADD CONSTRAINT chk_staff_message_threads_kind CHECK (kind IN ('direct', 'group'));

		ALTER TABLE users.staff_message_threads
			DROP CONSTRAINT IF EXISTS chk_staff_message_threads_group_name;
		ALTER TABLE users.staff_message_threads
			ADD CONSTRAINT chk_staff_message_threads_group_name CHECK (
				(kind = 'direct' AND name IS NULL)
				OR (kind = 'group' AND name IS NOT NULL AND length(btrim(name)) > 0)
			);

		ALTER TABLE users.staff_message_participants
			ADD COLUMN IF NOT EXISTS history_from TIMESTAMPTZ;
	`); err != nil {
		return fmt.Errorf("error extending staff messaging tables for group chats: %w", err)
	}
	return nil
}

func staffGroupChatsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `
		DELETE FROM users.staff_message_threads WHERE kind = 'group';

		ALTER TABLE users.staff_message_participants DROP COLUMN IF EXISTS history_from;

		ALTER TABLE users.staff_message_threads
			DROP CONSTRAINT IF EXISTS chk_staff_message_threads_group_name;
		ALTER TABLE users.staff_message_threads
			DROP CONSTRAINT IF EXISTS chk_staff_message_threads_kind;
		ALTER TABLE users.staff_message_threads
			ADD CONSTRAINT chk_staff_message_threads_kind CHECK (kind IN ('direct'));

		ALTER TABLE users.staff_message_threads
			DROP COLUMN IF EXISTS created_by_account_id,
			DROP COLUMN IF EXISTS name;
	`); err != nil {
		return fmt.Errorf("error removing staff group chat columns: %w", err)
	}
	return nil
}
