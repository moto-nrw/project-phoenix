package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	parentMessagingRequestsVersion     = "1.15.155"
	parentMessagingRequestsDescription = "Extend parent messaging rows with events and structured requests"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentMessagingRequestsVersion,
		Description: parentMessagingRequestsDescription,
		DependsOn:   []string{"1.15.149"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return parentMessagingRequestsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return parentMessagingRequestsDown(ctx, db)
		},
	)
}

func parentMessagingRequestsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.155: Extending parent messaging for events and requests...")

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
		ALTER TABLE users.parent_messages
			ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'message',
			ADD COLUMN IF NOT EXISTS event_type TEXT,
			ADD COLUMN IF NOT EXISTS event_actor_kind TEXT,
			ADD COLUMN IF NOT EXISTS request_type TEXT,
			ADD COLUMN IF NOT EXISTS request_status TEXT,
			ADD COLUMN IF NOT EXISTS payload JSONB,
			ADD COLUMN IF NOT EXISTS ref_table TEXT,
			ADD COLUMN IF NOT EXISTS ref_id BIGINT,
			ADD COLUMN IF NOT EXISTS applied_at TIMESTAMPTZ,
			ADD COLUMN IF NOT EXISTS applied_by BIGINT REFERENCES auth.accounts(id),
			ADD COLUMN IF NOT EXISTS decision_reason TEXT;

		ALTER TABLE users.parent_messages
			DROP CONSTRAINT IF EXISTS chk_parent_messages_kind,
			ADD CONSTRAINT chk_parent_messages_kind CHECK (kind IN ('message','event','request'));

		ALTER TABLE users.parent_messages
			DROP CONSTRAINT IF EXISTS chk_parent_messages_sender_kind,
			ADD CONSTRAINT chk_parent_messages_sender_kind CHECK (sender_kind IN ('guardian','staff','system'));

		ALTER TABLE users.parent_messages
			DROP CONSTRAINT IF EXISTS chk_parent_messages_event_actor_kind,
			ADD CONSTRAINT chk_parent_messages_event_actor_kind
				CHECK (event_actor_kind IS NULL OR event_actor_kind IN ('guardian','staff'));

		ALTER TABLE users.parent_messages
			DROP CONSTRAINT IF EXISTS chk_parent_messages_request_status,
			ADD CONSTRAINT chk_parent_messages_request_status
				CHECK (request_status IS NULL OR request_status IN ('offen','erledigt','abgelehnt','zurueckgezogen'));

		-- Partial index serving the inbox open_request_count subquery and the
		-- openRequestExists filter: both look up open requests by thread_id. A
		-- request is open only while request_status='offen' (confirm/reject pending).
		CREATE INDEX IF NOT EXISTS idx_parent_messages_open_requests
			ON users.parent_messages (tenant_id, thread_id, created_at DESC)
			WHERE kind='request' AND request_status = 'offen';
	`)
	if err != nil {
		return fmt.Errorf("error extending users.parent_messages: %w", err)
	}

	return tx.Commit()
}

func parentMessagingRequestsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.155: Removing parent messaging request fields...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// The up-migration widened sender_kind to allow 'system' and the app writes
	// 'system' rows (status events) from then on. Re-adding the narrow
	// ('guardian','staff') constraint while those rows exist raises
	// check_violation and aborts the whole rollback — exactly the recovery path
	// deploy-remote.sh relies on. Delete the now-orphaned event/request rows
	// (they only exist because of this migration's columns, which we are about
	// to drop anyway) before narrowing the constraint back.
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS users.idx_parent_messages_open_requests;
		ALTER TABLE users.parent_messages
			DROP CONSTRAINT IF EXISTS chk_parent_messages_request_status,
			DROP CONSTRAINT IF EXISTS chk_parent_messages_event_actor_kind,
			DROP CONSTRAINT IF EXISTS chk_parent_messages_kind;
		DELETE FROM users.parent_messages WHERE sender_kind = 'system';
		-- A confirm/reject/withdrawal system event is touched onto the thread via
		-- TouchLastMessage, so when it was the latest row the denormalized preview
		-- columns (last_message_at, last_message_id, last_sender_kind,
		-- last_message_body) now point at a just-deleted message. The pre-requests
		-- inbox reads those columns directly for previews and ordering, so leaving
		-- them dangling shows a non-existent "latest message" until another chat
		-- message touches the thread. Re-derive them from the newest SURVIVING
		-- message (created_at DESC, id DESC — the inbox projection's ordering),
		-- restricted to threads whose pointer actually moved.
		UPDATE users.parent_message_threads t
		SET last_message_at   = lm.created_at,
		    last_message_id   = lm.id,
		    last_sender_kind  = lm.sender_kind,
		    last_message_body = lm.body
		FROM (
			SELECT DISTINCT ON (m.thread_id)
			       m.thread_id, m.created_at, m.id, m.sender_kind, m.body
			FROM users.parent_messages m
			ORDER BY m.thread_id, m.created_at DESC, m.id DESC
		) lm
		WHERE lm.thread_id = t.id
		  AND (t.last_message_id IS NULL OR t.last_message_id <> lm.id);
		ALTER TABLE users.parent_messages
			DROP CONSTRAINT IF EXISTS chk_parent_messages_sender_kind,
			ADD CONSTRAINT chk_parent_messages_sender_kind CHECK (sender_kind IN ('guardian','staff'));
		ALTER TABLE users.parent_messages
			DROP COLUMN IF EXISTS decision_reason,
			DROP COLUMN IF EXISTS applied_by,
			DROP COLUMN IF EXISTS applied_at,
			DROP COLUMN IF EXISTS ref_id,
			DROP COLUMN IF EXISTS ref_table,
			DROP COLUMN IF EXISTS payload,
			DROP COLUMN IF EXISTS request_status,
			DROP COLUMN IF EXISTS request_type,
			DROP COLUMN IF EXISTS event_actor_kind,
			DROP COLUMN IF EXISTS event_type,
			DROP COLUMN IF EXISTS kind;
	`)
	if err != nil {
		return fmt.Errorf("error removing parent messaging request fields: %w", err)
	}

	return tx.Commit()
}
