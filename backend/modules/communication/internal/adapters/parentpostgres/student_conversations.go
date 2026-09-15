package parentpostgres

import (
	"context"
	"fmt"
)

// StudentConversationStore answers what a permanent child deletion has to
// account for in Communication's conversation tables, and takes the lock that
// keeps that answer valid until the deletion commits.
type StudentConversationStore struct{ store }

// NewStudentConversationStore builds the deletion-support store over the
// ambient transaction runtime.
func NewStudentConversationStore(database Database) *StudentConversationStore {
	return &StudentConversationStore{store: newStore(database)}
}

// CountRecords returns how many conversation rows belong to one child: its
// threads, their messages, and every read cursor into them.
func (s *StudentConversationStore) CountRecords(ctx context.Context, studentID int64) (int, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, err
	}
	if tenantID <= 0 {
		return 0, fmt.Errorf("count student conversation records: tenant context is required")
	}
	var counts struct {
		Records int `bun:"records"`
	}
	if err := db.NewRaw(`
		SELECT (
			(SELECT COUNT(*) FROM users.parent_message_threads WHERE tenant_id = ? AND student_id = ?) +
			(SELECT COUNT(*) FROM users.parent_messages WHERE tenant_id = ? AND student_id = ?) +
			(
				SELECT COUNT(*)
				FROM users.parent_message_reads AS "parent_message_read"
				JOIN users.parent_message_threads AS "parent_message_thread"
					ON "parent_message_thread".id = "parent_message_read".thread_id
					AND "parent_message_thread".tenant_id = "parent_message_read".tenant_id
				WHERE "parent_message_read".tenant_id = ? AND "parent_message_thread".student_id = ?
			)
		)::int AS records`,
		tenantID, studentID, tenantID, studentID, tenantID, studentID,
	).Scan(ctx, &counts); err != nil {
		return 0, fmt.Errorf("count student conversation records: %w", err)
	}
	return counts.Records, nil
}

// LockThreads prevents a read cursor or message from being added after the
// deletion preview is rechecked. Both row kinds reference their thread, so the
// FOR UPDATE lock serializes their foreign-key checks until the deletion
// commits or rolls back.
func (s *StudentConversationStore) LockThreads(ctx context.Context, studentID int64) error {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	if tenantID <= 0 {
		return fmt.Errorf("lock student message threads: tenant context is required")
	}
	var threadIDs []int64
	if err := db.NewSelect().
		TableExpr(parentThreadTableExpr).
		ColumnExpr(`"parent_message_thread".id`).
		Where(`"parent_message_thread".tenant_id = ?`, tenantID).
		Where(`"parent_message_thread".student_id = ?`, studentID).
		OrderExpr(`"parent_message_thread".id ASC`).
		For("UPDATE").
		Scan(ctx, &threadIDs); err != nil {
		return fmt.Errorf("lock student message threads: %w", err)
	}
	return nil
}
