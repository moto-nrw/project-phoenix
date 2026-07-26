package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	migrateOpenCareRequestsVersion     = "1.15.165"
	migrateOpenCareRequestsDescription = "Move open chat care_schedule requests into schedule.care_schedule_change_requests, convert their chat rows to notification pills (#1803)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     migrateOpenCareRequestsVersion,
		Description: migrateOpenCareRequestsDescription,
		DependsOn: []string{
			careScheduleChangeRequestsVersion, // the target table
			parentMessagingRequestsVersion,    // the source request columns on users.parent_messages
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return migrateOpenCareRequestsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return migrateOpenCareRequestsDown(ctx, db)
		},
	)
}

// migrateOpenCareRequestsUp is the #1803 cutover. Care-schedule change
// requests used to live as kind='request' rows in users.parent_messages,
// created and decided in the chat. The lifecycle moved to
// schedule.care_schedule_change_requests (decided on the Änderungsanfragen
// page); the chat keeps only non-interactive pills.
//
//  1. Dedup: the chat allowed several open requests per child (one per
//     guardian thread, or stacked); the new table enforces exactly ONE
//     pending per student. Keep the newest open request per (tenant,
//     student), flip older ones to 'zurueckgezogen' (the parent effectively
//     superseded them).
//  2. Move: insert the surviving open requests as 'pending' rows, preserving
//     created_at and the submitting guardian.
//  3. Convert: the moved chat rows become 'request_created' notification
//     pills (kind='event') referencing the new row — matching the pills the
//     new create path emits, so the timeline stays coherent. The payload is
//     cleared (it lives on the new row now). TERMINAL chat requests
//     (erledigt/abgelehnt/zurueckgezogen) stay untouched as read-only
//     history.
//  4. Repair previews: a converted row may have been a thread's
//     last_message_id, but request_created pills are deliberately excluded from
//     the denormalized last_message_* preview/ordering. Re-derive those columns
//     from the newest previewable message per affected thread (or clear them
//     where none remains) so migrated tenants don't see a stale Nachrichten
//     preview pointing at a now-hidden pill.
//
// Runs as the superuser CLI connection, so RLS is bypassed and one statement
// covers all tenants.
func migrateOpenCareRequestsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.165: Moving open care_schedule chat requests to schedule.care_schedule_change_requests...")

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
		-- 1) Keep only the newest open request per (tenant, student); flip the
		--    rest to withdrawn so step 2 cannot violate the one-pending-per-
		--    student unique index.
		WITH ranked AS (
			SELECT id,
			       ROW_NUMBER() OVER (
			           PARTITION BY tenant_id, student_id
			           ORDER BY created_at DESC, id DESC
			       ) AS rn
			FROM users.parent_messages
			WHERE kind = 'request'
			  AND request_type = 'care_schedule'
			  AND request_status = 'offen'
		)
		UPDATE users.parent_messages m
		SET request_status = 'zurueckgezogen',
		    updated_at = NOW()
		FROM ranked r
		WHERE m.id = r.id
		  AND r.rn > 1;

		-- 2) Move the surviving open requests into the schedule domain.
		INSERT INTO schedule.care_schedule_change_requests
			(tenant_id, student_id, submitted_by, payload, status, created_at, updated_at)
		SELECT m.tenant_id, m.student_id, m.sender_account_id, m.payload, 'pending', m.created_at, NOW()
		FROM users.parent_messages m
		WHERE m.kind = 'request'
		  AND m.request_type = 'care_schedule'
		  AND m.request_status = 'offen'
		  AND NOT EXISTS (
			SELECT 1 FROM schedule.care_schedule_change_requests r
			WHERE r.tenant_id = m.tenant_id
			  AND r.student_id = m.student_id
			  AND r.status = 'pending'
		  );

		-- 3) Convert the moved chat rows into 'request_created' pills
		--    referencing the new row (each (tenant, student) has exactly one
		--    pending row after step 2). sender becomes 'system' to match the
		--    pills the new create path emits; request_status stays 'offen'
		--    (the created-pill convention).
		UPDATE users.parent_messages m
		SET kind = 'event',
		    event_type = 'request_created',
		    event_actor_kind = 'guardian',
		    sender_kind = 'system',
		    sender_name = 'System',
		    payload = NULL,
		    ref_table = 'schedule.care_schedule_change_requests',
		    ref_id = r.id,
		    updated_at = NOW()
		FROM schedule.care_schedule_change_requests r
		WHERE r.tenant_id = m.tenant_id
		  AND r.student_id = m.student_id
		  AND r.status = 'pending'
		  AND m.kind = 'request'
		  AND m.request_type = 'care_schedule'
		  AND m.request_status = 'offen';

		-- 4) Repair denormalized thread previews. Step 3 may have converted a
		--    row that was a thread's last_message_id into a 'request_created'
		--    pill, which the app deliberately keeps OUT of the preview and
		--    ordering (see services/parentmessaging/emitter.go and
		--    counterpartUnread). A thread still pointing at such a pill would
		--    render a stale "latest message" in the Nachrichten inbox until the
		--    next real message touches it. Re-derive last_message_* from the
		--    newest PREVIEWABLE message (anything but a request_created pill,
		--    mirroring the unread-count exclusion), attributing it to the
		--    EFFECTIVE actor kind so the thread's
		--    last_sender_kind IN ('guardian','staff') check holds; where no
		--    previewable message remains, clear the columns back to the
		--    fresh-thread empty state (NULL activity, '' body). The pre-migration
		--    app never points last_message_id at a request_created pill, so this
		--    targets exactly the rows step 3 just touched and is idempotent.
		WITH affected AS (
			SELECT t.id AS thread_id
			FROM users.parent_message_threads t
			JOIN users.parent_messages lm
			  ON lm.id = t.last_message_id
			 AND lm.tenant_id = t.tenant_id
			WHERE lm.event_type = 'request_created'
		),
		newest AS (
			SELECT DISTINCT ON (m.thread_id)
			       m.thread_id,
			       m.created_at,
			       m.id,
			       CASE WHEN m.sender_kind IN ('guardian','staff') THEN m.sender_kind
			            ELSE m.event_actor_kind END AS actor_kind,
			       m.body
			FROM users.parent_messages m
			JOIN affected a ON a.thread_id = m.thread_id
			WHERE m.event_type IS DISTINCT FROM 'request_created'
			ORDER BY m.thread_id, m.created_at DESC, m.id DESC
		)
		UPDATE users.parent_message_threads t
		SET last_message_at   = newest.created_at,
		    last_message_id   = newest.id,
		    last_sender_kind  = newest.actor_kind,
		    last_message_body = COALESCE(newest.body, ''),
		    updated_at        = NOW()
		FROM affected a
		LEFT JOIN newest ON newest.thread_id = a.thread_id
		WHERE t.id = a.thread_id;
	`)
	if err != nil {
		return fmt.Errorf("error migrating open care_schedule requests: %w", err)
	}

	return tx.Commit()
}

// migrateOpenCareRequestsDown best-effort reverses the cutover: pending rows
// in the new table whose creation pill still points at them are turned back
// into open chat requests (payload restored from the new row), then deleted.
// The step-1 dedup withdrawals are NOT reverted (indistinguishable from
// genuine withdrawals) and pills for rows created through the NEW code path
// after the cutover revert to chat requests as well — acceptable for a dev
// rollback.
func migrateOpenCareRequestsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.165: Restoring open care requests into users.parent_messages...")

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
		UPDATE users.parent_messages m
		SET kind = 'request',
		    event_type = NULL,
		    event_actor_kind = NULL,
		    sender_kind = 'guardian',
		    payload = r.payload,
		    ref_table = NULL,
		    ref_id = NULL,
		    updated_at = NOW()
		FROM schedule.care_schedule_change_requests r
		WHERE m.ref_table = 'schedule.care_schedule_change_requests'
		  AND m.ref_id = r.id
		  AND m.event_type = 'request_created'
		  AND r.status = 'pending';

		DELETE FROM schedule.care_schedule_change_requests WHERE status = 'pending';
	`)
	if err != nil {
		return fmt.Errorf("error restoring open care requests: %w", err)
	}

	return tx.Commit()
}
