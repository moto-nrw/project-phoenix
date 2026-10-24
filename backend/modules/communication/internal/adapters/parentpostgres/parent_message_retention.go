package parentpostgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

// RetentionStore deletes the parent/OGS messages of a school past their
// retention window (#3959). It runs in the tenant transaction of the nightly
// cleanup and is the only path that deletes parent messages.
type RetentionStore struct{ store }

// NewRetentionStore builds the retention store over the ambient transaction
// runtime.
func NewRetentionStore(database Database) *RetentionStore {
	return &RetentionStore{store: newStore(database)}
}

// deleteExpiredParentMessagesSQL removes the messages written before the
// cutoff. An open request stays: the OGS has not answered it yet, and its
// decision still needs the message.
const deleteExpiredParentMessagesSQL = `DELETE FROM users.parent_messages
WHERE tenant_id = ?
  AND created_at < ?
  AND (kind <> 'request' OR request_status IS DISTINCT FROM 'offen')`

// repairThreadPreviewSQL recomputes the inbox preview of every thread whose
// last message was just deleted, from its newest remaining message the
// preview counts. Without it the thread would keep showing the deleted text
// in last_message_body. A thread with no such message falls back to the
// empty preview the inbox renders for a thread without activity.
const repairThreadPreviewSQL = `UPDATE users.parent_message_threads AS thread
SET last_message_id = latest.id,
    last_message_at = latest.created_at,
    last_sender_kind = latest.sender_kind,
    last_message_body = COALESCE(latest.body, '')
FROM users.parent_message_threads AS stale
LEFT JOIN LATERAL (
    SELECT message.id, message.created_at, message.body,
           CASE WHEN message.kind = 'event' THEN message.event_actor_kind ELSE message.sender_kind END AS sender_kind
    FROM users.parent_messages AS message
    WHERE message.thread_id = stale.id
      AND NOT (message.kind = 'event' AND message.event_type = 'request_created')
      AND (CASE WHEN message.kind = 'event' THEN message.event_actor_kind ELSE message.sender_kind END) IN ('guardian', 'staff')
    ORDER BY message.created_at DESC, message.id DESC
    LIMIT 1
) AS latest ON TRUE
WHERE thread.id = stale.id
  AND stale.tenant_id = ?
  AND stale.last_message_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM users.parent_messages AS kept WHERE kept.id = stale.last_message_id)`

// deleteEmptyParentThreadsSQL removes the conversations without any message
// left. A thread opened after the cutoff stays: it was just started and its
// first message may still follow. Read cursors go with the thread.
const deleteEmptyParentThreadsSQL = `DELETE FROM users.parent_message_threads AS thread
WHERE thread.tenant_id = ?
  AND thread.created_at < ?
  AND NOT EXISTS (SELECT 1 FROM users.parent_messages AS message WHERE message.thread_id = thread.id)`

// deleteExpiredGroupMessagesSQL removes the group thread messages (#4784)
// written before the cutoff; their recipient rows go with them.
const deleteExpiredGroupMessagesSQL = `DELETE FROM users.parent_group_messages
WHERE tenant_id = ?
  AND created_at < ?`

// deleteEmptyGroupThreadsSQL removes the group threads without any message
// left. Their read cursors go with the thread; answers in the 1:1
// conversations keep their text and lose only the reference.
const deleteEmptyGroupThreadsSQL = `DELETE FROM users.parent_group_threads AS thread
WHERE thread.tenant_id = ?
  AND thread.created_at < ?
  AND NOT EXISTS (SELECT 1 FROM users.parent_group_messages AS message WHERE message.thread_id = thread.id)`

// DeleteMessagesBefore removes the school's messages written before cutoff,
// repairs the previews they leave behind and removes the conversations left
// empty. It reports how many messages and conversations went.
func (s *RetentionStore) DeleteMessagesBefore(ctx context.Context, cutoff time.Time) (messages, threads int64, err error) {
	if cutoff.IsZero() {
		return 0, 0, errors.New("delete expired parent messages: cutoff is required")
	}
	db, scope, err := s.database(ctx)
	if err != nil {
		return 0, 0, err
	}
	if scope.TenantID <= 0 {
		return 0, 0, errors.New("delete expired parent messages: tenant is required")
	}
	result, err := db.ExecContext(ctx, deleteExpiredParentMessagesSQL, scope.TenantID, cutoff)
	if err != nil {
		return 0, 0, fmt.Errorf("delete expired parent messages: %w", err)
	}
	if messages, err = result.RowsAffected(); err != nil {
		return 0, 0, fmt.Errorf("count expired parent messages: %w", err)
	}
	if _, err := db.ExecContext(ctx, repairThreadPreviewSQL, scope.TenantID); err != nil {
		return 0, 0, fmt.Errorf("repair parent message thread previews: %w", err)
	}
	result, err = db.ExecContext(ctx, deleteEmptyParentThreadsSQL, scope.TenantID, cutoff)
	if err != nil {
		return 0, 0, fmt.Errorf("delete empty parent message threads: %w", err)
	}
	if threads, err = result.RowsAffected(); err != nil {
		return 0, 0, fmt.Errorf("count empty parent message threads: %w", err)
	}
	groupMessages, groupThreads, err := deleteExpiredGroupThreads(ctx, db, scope.TenantID, cutoff)
	if err != nil {
		return 0, 0, err
	}
	return messages + groupMessages, threads + groupThreads, nil
}

// deleteExpiredGroupThreads applies the same retention to the group threads.
func deleteExpiredGroupThreads(ctx context.Context, db bun.IDB, tenantID int64, cutoff time.Time) (messages, threads int64, err error) {
	result, err := db.ExecContext(ctx, deleteExpiredGroupMessagesSQL, tenantID, cutoff)
	if err != nil {
		return 0, 0, fmt.Errorf("delete expired parent group messages: %w", err)
	}
	if messages, err = result.RowsAffected(); err != nil {
		return 0, 0, fmt.Errorf("count expired parent group messages: %w", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE users.parent_group_threads AS thread
SET last_message_at = (
	SELECT MAX(message.created_at)
	FROM users.parent_group_messages AS message
	WHERE message.thread_id = thread.id
	  AND message.tenant_id = thread.tenant_id
)
WHERE thread.tenant_id = ?
	AND thread.last_message_at IS DISTINCT FROM (
		SELECT MAX(message.created_at)
		FROM users.parent_group_messages AS message
		WHERE message.thread_id = thread.id
		  AND message.tenant_id = thread.tenant_id
	)`, tenantID); err != nil {
		return 0, 0, fmt.Errorf("repair parent group thread activity: %w", err)
	}
	result, err = db.ExecContext(ctx, deleteEmptyGroupThreadsSQL, tenantID, cutoff)
	if err != nil {
		return 0, 0, fmt.Errorf("delete empty parent group threads: %w", err)
	}
	if threads, err = result.RowsAffected(); err != nil {
		return 0, 0, fmt.Errorf("count empty parent group threads: %w", err)
	}
	return messages, threads, nil
}
