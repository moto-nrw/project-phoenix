package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	parentMessagingVersion     = "1.15.137"
	parentMessagingDescription = "Create parent-OGS messaging tables (threads, messages, reads) and backfill from student_parent_notes"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentMessagingVersion,
		Description: parentMessagingDescription,
		DependsOn: []string{
			"1.15.114", // users.student_parent_notes (backfill source)
			"1.3.5",    // users.students
			"1.0.1",    // auth.accounts
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return parentMessagingUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return parentMessagingDown(ctx, db)
		},
	)
}

func parentMessagingUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.137: Creating parent messaging tables...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// A thread is one conversation between the OGS and a SINGLE guardian
	// about one child — like an email thread. A guardian can have several
	// threads about the same child (distinguished by subject), so there is
	// no per-child uniqueness. last_sender_kind lets the inbox flag
	// "awaiting OGS reply" without scanning the messages.
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.parent_message_threads (
			id                  BIGSERIAL PRIMARY KEY,
			tenant_id           BIGINT NOT NULL REFERENCES platform.schools(id),
			student_id          BIGINT NOT NULL REFERENCES users.students(id) ON DELETE CASCADE,
			guardian_account_id BIGINT NOT NULL REFERENCES auth.accounts(id),
			subject             TEXT NOT NULL,
			last_message_at     TIMESTAMPTZ,
			last_sender_kind    TEXT,
			created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_parent_message_threads_subject_not_blank CHECK (length(btrim(subject)) > 0),
			CONSTRAINT chk_parent_message_threads_last_sender CHECK (last_sender_kind IN ('guardian','staff'))
		);

		CREATE INDEX IF NOT EXISTS idx_parent_message_threads_inbox
			ON users.parent_message_threads (tenant_id, last_message_at DESC);
		CREATE INDEX IF NOT EXISTS idx_parent_message_threads_guardian
			ON users.parent_message_threads (tenant_id, guardian_account_id, last_message_at DESC);
		CREATE INDEX IF NOT EXISTS idx_parent_message_threads_student
			ON users.parent_message_threads (tenant_id, student_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating users.parent_message_threads: %w", err)
	}

	// Append-only message log. sender_name is denormalized at send time so
	// the chat history stays stable even if an account/profile is renamed
	// or removed, and reads need no joins. student_id is denormalized for
	// the inbox ACL filter (student -> group).
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.parent_messages (
			id                BIGSERIAL PRIMARY KEY,
			tenant_id         BIGINT NOT NULL REFERENCES platform.schools(id),
			thread_id         BIGINT NOT NULL REFERENCES users.parent_message_threads(id) ON DELETE CASCADE,
			student_id        BIGINT NOT NULL REFERENCES users.students(id) ON DELETE CASCADE,
			sender_account_id BIGINT NOT NULL REFERENCES auth.accounts(id),
			sender_kind       TEXT NOT NULL,
			sender_name       TEXT NOT NULL,
			body              TEXT NOT NULL,
			created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_parent_messages_sender_kind CHECK (sender_kind IN ('guardian','staff')),
			CONSTRAINT chk_parent_messages_body_not_blank CHECK (length(btrim(body)) > 0)
		);

		CREATE INDEX IF NOT EXISTS idx_parent_messages_thread_created
			ON users.parent_messages (tenant_id, thread_id, created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_parent_messages_student_created
			ON users.parent_messages (tenant_id, student_id, created_at DESC);
	`)
	if err != nil {
		return fmt.Errorf("error creating users.parent_messages: %w", err)
	}

	// Per-reader read cursor (account = guardian or staff). Unread is any
	// message after last_read_at not sent by the reader.
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.parent_message_reads (
			tenant_id    BIGINT NOT NULL REFERENCES platform.schools(id),
			thread_id    BIGINT NOT NULL REFERENCES users.parent_message_threads(id) ON DELETE CASCADE,
			account_id   BIGINT NOT NULL REFERENCES auth.accounts(id),
			last_read_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (thread_id, account_id)
		);

		CREATE INDEX IF NOT EXISTS idx_parent_message_reads_account
			ON users.parent_message_reads (tenant_id, account_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating users.parent_message_reads: %w", err)
	}

	// updated_at triggers
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_parent_message_threads_updated_at ON users.parent_message_threads;
		CREATE TRIGGER update_parent_message_threads_updated_at
		BEFORE UPDATE ON users.parent_message_threads
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		DROP TRIGGER IF EXISTS update_parent_messages_updated_at ON users.parent_messages;
		CREATE TRIGGER update_parent_messages_updated_at
		BEFORE UPDATE ON users.parent_messages
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at triggers: %w", err)
	}

	// RLS — tenant isolation, same shape as student_parent_notes.
	for _, table := range []string{"parent_message_threads", "parent_messages", "parent_message_reads"} {
		_, err = tx.ExecContext(ctx, fmt.Sprintf(`
			ALTER TABLE users.%[1]s ENABLE ROW LEVEL SECURITY;
			ALTER TABLE users.%[1]s FORCE ROW LEVEL SECURITY;

			CREATE POLICY tenant_isolation_users_%[1]s ON users.%[1]s
				FOR ALL
				USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
				WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

			GRANT SELECT, INSERT, UPDATE, DELETE ON users.%[1]s TO phoenix_tenant;
		`, table))
		if err != nil {
			return fmt.Errorf("error enabling RLS on users.%s: %w", table, err)
		}
	}

	_, err = tx.ExecContext(ctx, `
		GRANT USAGE ON SEQUENCE users.parent_message_threads_id_seq TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE users.parent_messages_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error granting sequence usage: %w", err)
	}

	// Backfill from existing parent notes. The migration runs as the
	// superuser (bypasses RLS), so it sees every tenant's notes. One thread
	// per (tenant, student, guardian) — each guardian's notes become their
	// own conversation under a generic subject; each note becomes a guardian
	// message with the guardian's name resolved from their profile.
	_, err = tx.ExecContext(ctx, `
		INSERT INTO users.parent_message_threads
			(tenant_id, student_id, guardian_account_id, subject, last_message_at, last_sender_kind, created_at, updated_at)
		SELECT n.tenant_id, n.student_id, n.guardian_account_id, 'Elternnachrichten',
		       MAX(n.created_at), 'guardian', MIN(n.created_at), MAX(n.created_at)
		FROM users.student_parent_notes n
		GROUP BY n.tenant_id, n.student_id, n.guardian_account_id;

		INSERT INTO users.parent_messages
			(tenant_id, thread_id, student_id, sender_account_id, sender_kind, sender_name, body, created_at, updated_at)
		SELECT n.tenant_id, t.id, n.student_id, n.guardian_account_id, 'guardian',
		       COALESCE(NULLIF(btrim(COALESCE(gp.first_name,'') || ' ' || COALESCE(gp.last_name,'')), ''), 'Elternteil'),
		       n.body, n.created_at, n.updated_at
		FROM users.student_parent_notes n
		JOIN users.parent_message_threads t
			ON t.tenant_id = n.tenant_id
			AND t.student_id = n.student_id
			AND t.guardian_account_id = n.guardian_account_id
		LEFT JOIN users.guardian_profiles gp ON gp.account_id = n.guardian_account_id;
	`)
	if err != nil {
		return fmt.Errorf("error backfilling parent messages from notes: %w", err)
	}

	return tx.Commit()
}

func parentMessagingDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.137: Dropping parent messaging tables...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_parent_messages_updated_at ON users.parent_messages;
		DROP TRIGGER IF EXISTS update_parent_message_threads_updated_at ON users.parent_message_threads;
		DROP TABLE IF EXISTS users.parent_message_reads CASCADE;
		DROP TABLE IF EXISTS users.parent_messages CASCADE;
		DROP TABLE IF EXISTS users.parent_message_threads CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping parent messaging tables: %w", err)
	}

	return tx.Commit()
}
