package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/uptrace/bun"
)

const (
	staffMessagingVersion     = "1.15.332"
	staffMessagingDescription = "Create OGS-internal staff messaging tables (threads, participants, messages, reads) for the 1:1 colleague chat (#2598)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffMessagingVersion,
		Description: staffMessagingDescription,
		DependsOn: []string{
			AuthAccountsVersion,         // auth.accounts (participants, senders, read cursors)
			createAccountTenantsVersion, // auth.account_tenants (who may be addressed)
		},
	})

	Migrations.MustRegister(staffMessagingUp, staffMessagingDown)
}

// staffMessagingTables is the RLS/GRANT loop's subject list, newest-dependency
// last so the DROP in the down migration can walk it in reverse.
var staffMessagingTables = []string{
	"staff_message_threads",
	"staff_message_participants",
	"staff_messages",
	"staff_message_reads",
}

func staffMessagingUp(ctx context.Context, db *bun.DB) error {
	slog.Info("migration starting",
		slog.String("migration", staffMessagingVersion),
		slog.String("detail", "creating staff messaging tables"),
	)

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			// slog, not log.Printf: the repository is slog-only (backend/CLAUDE.md).
			// Much of this package still predates that rule; new files do not add to
			// the backlog.
			slog.Error("rollback failed",
				slog.String("migration", staffMessagingVersion),
				slog.String("error", err.Error()),
			)
		}
	}()

	// A thread is ONE continuous conversation between staff accounts of a single
	// school. participant_key identifies the conversation independently of how
	// many people are in it: for a direct chat it is the two account ids sorted
	// ascending and joined by a colon ("17:42"), which makes the send path a
	// get-or-create against a single unique index.
	//
	// The key is deliberately NOT two account columns. V1 ships 1:1 only, but a
	// later group chat then needs a different key (and rows in the participants
	// table) rather than a data migration of this table — see #2598. `kind`
	// names the shape so a reader does not have to parse the key to know it.
	//
	// last_* denormalizes the inbox preview so listing threads does not run a
	// correlated subquery per row. Every path that advances last_message_at MUST
	// set last_message_id and last_message_body in the SAME update, because the
	// (created_at, id) composite is what the message list and the unread cursor
	// order by.
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.staff_message_threads (
			id                     BIGSERIAL PRIMARY KEY,
			tenant_id              BIGINT NOT NULL REFERENCES platform.schools(id),
			participant_key        TEXT NOT NULL,
			kind                   TEXT NOT NULL DEFAULT 'direct',
			last_message_at        TIMESTAMPTZ,
			last_message_id        BIGINT,
			last_message_body      TEXT NOT NULL DEFAULT '',
			last_sender_account_id BIGINT REFERENCES auth.accounts(id),
			created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_staff_message_threads_participants UNIQUE (tenant_id, participant_key),
			CONSTRAINT chk_staff_message_threads_kind CHECK (kind IN ('direct')),
			CONSTRAINT chk_staff_message_threads_key_not_blank CHECK (length(btrim(participant_key)) > 0)
		);

		CREATE INDEX IF NOT EXISTS idx_staff_message_threads_inbox
			ON users.staff_message_threads (tenant_id, last_message_at DESC);
	`)
	if err != nil {
		return fmt.Errorf("error creating users.staff_message_threads: %w", err)
	}

	// Who is in a conversation. For a direct chat this holds exactly the two
	// rows encoded in participant_key; it exists as its own table so the inbox
	// ("threads I am in") is an index lookup rather than a LIKE over the key,
	// and so group threads need no schema change later.
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.staff_message_participants (
			tenant_id  BIGINT NOT NULL REFERENCES platform.schools(id),
			thread_id  BIGINT NOT NULL REFERENCES users.staff_message_threads(id) ON DELETE CASCADE,
			account_id BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (thread_id, account_id)
		);

		CREATE INDEX IF NOT EXISTS idx_staff_message_participants_account
			ON users.staff_message_participants (tenant_id, account_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating users.staff_message_participants: %w", err)
	}

	// Append-only message log. sender_name is denormalized at send time so the
	// history stays readable after an account is renamed, deactivated, or its
	// person record is removed — the chat must not turn into "Unbekannt" rows
	// retroactively.
	//
	// created_at uses clock_timestamp(), NOT NOW(): NOW() is the transaction
	// timestamp, so the message would be dated to before the checks that precede
	// the insert. That widens the window in which a concurrent reader advances
	// its read cursor past a message that has not committed yet — silently
	// marking an unread message read. clock_timestamp() stamps at the insert
	// itself, near commit, shrinking the window to the tail of the transaction.
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.staff_messages (
			id                BIGSERIAL PRIMARY KEY,
			tenant_id         BIGINT NOT NULL REFERENCES platform.schools(id),
			thread_id         BIGINT NOT NULL REFERENCES users.staff_message_threads(id) ON DELETE CASCADE,
			sender_account_id BIGINT NOT NULL REFERENCES auth.accounts(id),
			sender_name       TEXT NOT NULL,
			body              TEXT NOT NULL,
			created_at        TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
			updated_at        TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
			CONSTRAINT chk_staff_messages_body_not_blank CHECK (length(btrim(body)) > 0)
		);

		CREATE INDEX IF NOT EXISTS idx_staff_messages_thread_created
			ON users.staff_messages (tenant_id, thread_id, created_at DESC, id DESC);
	`)
	if err != nil {
		return fmt.Errorf("error creating users.staff_messages: %w", err)
	}

	// Per-reader read cursor. Unread is any message after the cursor not sent by
	// the reader. The cursor is the COMPOSITE (last_read_at, last_read_message_id)
	// for the same reason the message index is: clock_timestamp() is
	// microsecond-precision, so two rapid sends can share a created_at. A
	// timestamp-only cursor would treat a tied message that committed after the
	// reader's snapshot as already read and drop it from the unread badge, so the
	// cursor and every unread predicate carry the same id tie-breaker.
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.staff_message_reads (
			tenant_id            BIGINT NOT NULL REFERENCES platform.schools(id),
			thread_id            BIGINT NOT NULL REFERENCES users.staff_message_threads(id) ON DELETE CASCADE,
			account_id           BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
			last_read_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			last_read_message_id BIGINT NOT NULL DEFAULT 0,
			PRIMARY KEY (thread_id, account_id)
		);

		CREATE INDEX IF NOT EXISTS idx_staff_message_reads_account
			ON users.staff_message_reads (tenant_id, account_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating users.staff_message_reads: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_staff_message_threads_updated_at ON users.staff_message_threads;
		CREATE TRIGGER update_staff_message_threads_updated_at
		BEFORE UPDATE ON users.staff_message_threads
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		DROP TRIGGER IF EXISTS update_staff_messages_updated_at ON users.staff_messages;
		CREATE TRIGGER update_staff_messages_updated_at
		BEFORE UPDATE ON users.staff_messages
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at triggers: %w", err)
	}

	// Least-privilege role needs DML on all four tables; the sequences back the
	// two BIGSERIAL primary keys.
	for _, table := range staffMessagingTables {
		if _, err = tx.ExecContext(ctx, fmt.Sprintf(
			`GRANT SELECT, INSERT, UPDATE, DELETE ON users.%s TO phoenix_tenant;`, table,
		)); err != nil {
			return fmt.Errorf("error granting on users.%s: %w", table, err)
		}
	}

	_, err = tx.ExecContext(ctx, `
		GRANT USAGE ON SEQUENCE users.staff_message_threads_id_seq TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE users.staff_messages_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error granting sequence usage: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Tenant isolation over the shared provisioning path (rls_provisioning.go)
	// instead of hand-copied ALTER/CREATE POLICY statements: it applies exactly
	// the contract migration 1.15.1 established and keeps policy shape and
	// naming identical across every tenant table. It takes the plain db handle,
	// hence after the commit above.
	if err := provisionTenantRLS(ctx, db,
		"users.staff_message_threads",
		"users.staff_message_participants",
		"users.staff_messages",
		"users.staff_message_reads",
	); err != nil {
		return err
	}

	slog.Info("migration completed",
		slog.String("migration", staffMessagingVersion),
		slog.String("detail", "staff messaging tables created"),
	)
	return nil
}

func staffMessagingDown(ctx context.Context, db *bun.DB) error {
	slog.Info("migration rolling back",
		slog.String("migration", staffMessagingVersion),
		slog.String("detail", "dropping staff messaging tables"),
	)

	_, err := db.NewRaw(`
		DROP TABLE IF EXISTS users.staff_message_reads;
		DROP TABLE IF EXISTS users.staff_messages;
		DROP TABLE IF EXISTS users.staff_message_participants;
		DROP TABLE IF EXISTS users.staff_message_threads;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("error dropping staff messaging tables: %w", err)
	}
	return nil
}
