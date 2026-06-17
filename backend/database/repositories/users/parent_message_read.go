package users

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// ParentMessageReadRepository tracks per-reader read cursors and answers the
// thread list / unread queries for both portals. Composite-key table, so it
// does not embed the generic base.Repository.
type ParentMessageReadRepository struct {
	db *bun.DB
}

// NewParentMessageReadRepository wires a fresh repository.
func NewParentMessageReadRepository(db *bun.DB) users.ParentMessageReadRepository {
	return &ParentMessageReadRepository{db: db}
}

// MarkRead upserts the reader's cursor for a thread to now().
func (r *ParentMessageReadRepository) MarkRead(ctx context.Context, tenantID, threadID, accountID int64) error {
	row := &users.ParentMessageRead{
		ThreadID:   threadID,
		AccountID:  accountID,
		LastReadAt: time.Now(),
	}
	row.SetTenantID(tenantID)

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(row).
		ModelTableExpr("users.parent_message_reads").
		On("CONFLICT (thread_id, account_id) DO UPDATE").
		Set("last_read_at = NOW()").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "mark parent message thread read", Err: err}
	}
	return nil
}

// inboxColumns are the InboxThread projection columns, parameterized by the
// "unread" sender side (the OTHER party relative to the reader): staff readers
// count unread guardian messages, guardian readers count unread staff messages.
func inboxSelect(q *bun.SelectQuery, accountID int64, unreadSenderKind string) *bun.SelectQuery {
	unreadSub := fmt.Sprintf(`(
		SELECT COUNT(*) FROM users.parent_messages cm
		WHERE cm.thread_id = t.id
		  AND cm.sender_kind = '%s'
		  AND cm.created_at > COALESCE(r.last_read_at, '1970-01-01'::timestamptz)
	) AS unread_count`, unreadSenderKind)

	return q.
		TableExpr("users.parent_message_threads AS t").
		ColumnExpr("t.id AS thread_id").
		ColumnExpr("t.subject AS subject").
		ColumnExpr("t.student_id AS student_id").
		ColumnExpr("btrim(COALESCE(pn.first_name,'') || ' ' || COALESCE(pn.last_name,'')) AS student_name").
		ColumnExpr("s.school_class AS school_class").
		ColumnExpr("s.group_id AS group_id").
		ColumnExpr("COALESCE(g.name,'') AS group_name").
		ColumnExpr("t.guardian_account_id AS guardian_account_id").
		ColumnExpr("btrim(COALESCE(gp.first_name,'') || ' ' || COALESCE(gp.last_name,'')) AS guardian_name").
		ColumnExpr("COALESCE(sg.relationship_type,'') AS relationship_type").
		ColumnExpr("t.last_message_at AS last_message_at").
		ColumnExpr("COALESCE(t.last_sender_kind,'') AS last_sender_kind").
		ColumnExpr(`COALESCE((
			SELECT lm.body FROM users.parent_messages lm
			WHERE lm.thread_id = t.id
			ORDER BY lm.created_at DESC, lm.id DESC LIMIT 1
		),'') AS last_message_body`).
		ColumnExpr(unreadSub).
		Join("JOIN users.students AS s ON s.id = t.student_id").
		Join("JOIN users.persons AS pn ON pn.id = s.person_id AND pn.deleted_at IS NULL").
		Join("LEFT JOIN education.groups AS g ON g.id = s.group_id").
		Join("LEFT JOIN users.guardian_profiles AS gp ON gp.account_id = t.guardian_account_id").
		Join("LEFT JOIN users.students_guardians AS sg ON sg.guardian_profile_id = gp.id AND sg.student_id = t.student_id").
		Join("LEFT JOIN users.parent_message_reads AS r ON r.thread_id = t.id AND r.account_id = ?", accountID).
		OrderExpr("t.last_message_at DESC NULLS LAST")
}

// guardianUnreadExists is the EXISTS predicate for "an unread guardian message"
// for the staff reader (used by the inbox onlyUnread filter and the badge).
const guardianUnreadExists = `EXISTS (
	SELECT 1 FROM users.parent_messages um
	WHERE um.thread_id = t.id
	  AND um.sender_kind = 'guardian'
	  AND um.created_at > COALESCE(r.last_read_at, '1970-01-01'::timestamptz)
)`

// applyStaffScope narrows a thread query to the students a staff member may
// read: all students (admin / all_staff scope) or only their supervised
// groups (empty groups → impossible filter, so the result is empty).
func applyStaffScope(q *bun.SelectQuery, allStudents bool, groupIDs []int64) *bun.SelectQuery {
	if allStudents {
		return q
	}
	if len(groupIDs) == 0 {
		return q.Where("1 = 0")
	}
	return q.Where("s.group_id IN (?)", bun.List(groupIDs))
}

// ListInboxForStaff returns the staff member's readable threads, newest
// activity first. onlyUnread keeps only threads with an unread guardian message.
func (r *ParentMessageReadRepository) ListInboxForStaff(ctx context.Context, accountID int64, allStudents bool, groupIDs []int64, onlyUnread bool) ([]*users.InboxThread, error) {
	var rows []*users.InboxThread
	query := inboxSelect(base.GetDB(ctx, r.db).NewSelect(), accountID, users.ParentMessageSenderGuardian)
	query = applyStaffScope(query, allStudents, groupIDs)
	if where, val, ok := base.TenantWhere(ctx, "t"); ok {
		query = query.Where(where, val)
	}
	if onlyUnread {
		query = query.Where(guardianUnreadExists)
	}
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list parent message inbox", Err: err}
	}
	return rows, nil
}

// ListThreadsForGuardian returns the guardian's own threads, newest activity
// first; unread counts staff messages (the side the guardian has not read).
func (r *ParentMessageReadRepository) ListThreadsForGuardian(ctx context.Context, accountID int64) ([]*users.InboxThread, error) {
	var rows []*users.InboxThread
	query := inboxSelect(base.GetDB(ctx, r.db).NewSelect(), accountID, users.ParentMessageSenderStaff).
		Where("t.guardian_account_id = ?", accountID)
	if where, val, ok := base.TenantWhere(ctx, "t"); ok {
		query = query.Where(where, val)
	}
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list guardian threads", Err: err}
	}
	return rows, nil
}

// FindThreadProjection returns the single-thread list projection used for the
// chat-window header, or nil when the thread does not exist / is out of tenant.
func (r *ParentMessageReadRepository) FindThreadProjection(ctx context.Context, threadID, accountID int64) (*users.InboxThread, error) {
	var rows []*users.InboxThread
	query := inboxSelect(base.GetDB(ctx, r.db).NewSelect(), accountID, users.ParentMessageSenderGuardian).
		Where("t.id = ?", threadID)
	if where, val, ok := base.TenantWhere(ctx, "t"); ok {
		query = query.Where(where, val)
	}
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find thread projection", Err: err}
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

// UnreadCountInThread counts messages from fromSenderKind created after the
// reader's cursor in a single thread.
func (r *ParentMessageReadRepository) UnreadCountInThread(ctx context.Context, threadID, accountID int64, fromSenderKind string) (int, error) {
	count, err := base.GetDB(ctx, r.db).NewSelect().
		TableExpr("users.parent_messages AS m").
		Join("LEFT JOIN users.parent_message_reads AS r ON r.thread_id = m.thread_id AND r.account_id = ?", accountID).
		Where("m.thread_id = ?", threadID).
		Where("m.sender_kind = ?", fromSenderKind).
		Where("m.created_at > COALESCE(r.last_read_at, '1970-01-01'::timestamptz)").
		Count(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "count unread parent messages in thread", Err: err}
	}
	return count, nil
}

// UnreadThreadCountForStaff counts threads with an unread guardian message
// for the staff reader, within their visible student scope.
func (r *ParentMessageReadRepository) UnreadThreadCountForStaff(ctx context.Context, accountID int64, allStudents bool, groupIDs []int64) (int, error) {
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr("users.parent_message_threads AS t").
		Join("JOIN users.students AS s ON s.id = t.student_id").
		Join("LEFT JOIN users.parent_message_reads AS r ON r.thread_id = t.id AND r.account_id = ?", accountID).
		Where(guardianUnreadExists)
	query = applyStaffScope(query, allStudents, groupIDs)
	if where, val, ok := base.TenantWhere(ctx, "t"); ok {
		query = query.Where(where, val)
	}
	count, err := query.Count(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "count unread parent message threads", Err: err}
	}
	return count, nil
}
