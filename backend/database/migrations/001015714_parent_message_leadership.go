package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	parentMessageLeadershipVersion     = "1.15.714"
	parentMessageLeadershipDescription = "Eltern schreiben wahlweise an das ganze Team oder nur an die Leitung: zweite Unterhaltung pro Kind und Elternteil, Recht parent_messages:leadership (#5200)"

	parentMessagesLeadershipPermissionName = "parent_messages:leadership"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentMessageLeadershipVersion,
		Description: parentMessageLeadershipDescription,
		DependsOn: []string{
			holidayGuestAnswerFilterVersion, // the last migration before this one
			parentMessagingVersion,          // creates users.parent_message_threads and its unique constraint
		},
	})
	Migrations.MustRegister(parentMessageLeadershipUp, parentMessageLeadershipDown)
}

// parentMessageLeadershipUpSQL gives every conversation its audience. A
// guardian keeps one conversation per child with the whole team and may open
// a second one that only the leadership reads, so the unique key gains the
// audience. Every existing conversation is a team conversation.
const parentMessageLeadershipUpSQL = `
		ALTER TABLE users.parent_message_threads
			ADD COLUMN IF NOT EXISTS audience TEXT NOT NULL DEFAULT 'team';

		ALTER TABLE users.parent_message_threads
			DROP CONSTRAINT IF EXISTS chk_parent_message_threads_audience,
			ADD CONSTRAINT chk_parent_message_threads_audience CHECK (audience IN ('team', 'leadership'));

		ALTER TABLE users.parent_message_threads
			DROP CONSTRAINT IF EXISTS uq_parent_message_threads_student_guardian,
			DROP CONSTRAINT IF EXISTS uq_parent_message_threads_student_guardian_audience,
			ADD CONSTRAINT uq_parent_message_threads_student_guardian_audience
				UNIQUE (tenant_id, student_id, guardian_account_id, audience);

		COMMENT ON COLUMN users.parent_message_threads.audience IS
			'team: every staff member who reads the child; leadership: only accounts holding parent_messages:leadership (#5200).';
`

// parentMessageLeadershipDownSQL restores one conversation per child and
// guardian. The caller has checked that no leadership conversation exists.
const parentMessageLeadershipDownSQL = `
		ALTER TABLE users.parent_message_threads
			DROP CONSTRAINT IF EXISTS uq_parent_message_threads_student_guardian_audience,
			DROP CONSTRAINT IF EXISTS uq_parent_message_threads_student_guardian,
			ADD CONSTRAINT uq_parent_message_threads_student_guardian
				UNIQUE (tenant_id, student_id, guardian_account_id);

		ALTER TABLE users.parent_message_threads
			DROP CONSTRAINT IF EXISTS chk_parent_message_threads_audience,
			DROP COLUMN IF EXISTS audience;
`

func parentMessageLeadershipUp(ctx context.Context, db *bun.DB) error {
	if err := db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, parentMessageLeadershipUpSQL); err != nil {
			return fmt.Errorf("add users.parent_message_threads.audience: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}
	return grantPermissionToRoles(ctx, db, permissionSpec{
		Name:        parentMessagesLeadershipPermissionName,
		Description: "Vertrauliche Elternnachrichten lesen, die nur an die Leitung gehen",
		Resource:    "parent_messages",
		Action:      "leadership",
	}, "admin")
}

// parentMessageLeadershipDown refuses while a leadership conversation exists:
// folding it into the team conversation would show the whole team what the
// parents addressed to the leadership only.
func parentMessageLeadershipDown(ctx context.Context, db *bun.DB) error {
	var leadership int
	if err := db.NewRaw(`SELECT COUNT(*) FROM users.parent_message_threads WHERE audience = 'leadership'`).
		Scan(ctx, &leadership); err != nil {
		return fmt.Errorf("count leadership parent message threads: %w", err)
	}
	if leadership > 0 {
		return fmt.Errorf("cannot drop users.parent_message_threads.audience while %d leadership conversations exist", leadership)
	}
	if err := db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, parentMessageLeadershipDownSQL); err != nil {
			return fmt.Errorf("drop users.parent_message_threads.audience: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}
	return dropPermission(ctx, db, parentMessagesLeadershipPermissionName)
}
