package parentpostgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/uptrace/bun"
)

// The guardian reads below see a message only through a recipient row that
// names the guardian AND one of the children the caller may still open
// (studentIDs, resolved from the live guardian permissions). Nothing they
// return describes another family.

// guardianGroupThreadListSQL lists the guardian's threads across the given
// schools, newest visible message first.
//
// Bind order: account, students, schools, account.
const guardianGroupThreadListSQL = `
WITH mine AS (
	SELECT r.tenant_id, r.thread_id, r.message_id, r.student_id
	FROM users.parent_group_message_recipients AS r
	WHERE r.guardian_account_id = ? AND r.student_id IN (?) AND r.tenant_id IN (?)
), per_thread AS (
	SELECT tenant_id, thread_id, MAX(message_id) AS last_message_id,
		array_agg(DISTINCT student_id ORDER BY student_id) AS student_ids
	FROM mine GROUP BY tenant_id, thread_id
)
SELECT gt.*, gm.body AS last_message_body, gm.created_at AS last_visible_at, pt.student_ids,
	(SELECT COUNT(DISTINCT m.message_id) FROM mine AS m
		WHERE m.thread_id = pt.thread_id AND m.message_id > COALESCE(rd.last_read_message_id, 0)) AS unread_count
FROM per_thread AS pt
JOIN users.parent_group_threads AS gt ON gt.id = pt.thread_id AND gt.tenant_id = pt.tenant_id
JOIN users.parent_group_messages AS gm ON gm.id = pt.last_message_id AND gm.tenant_id = pt.tenant_id
LEFT JOIN users.parent_group_thread_reads AS rd ON rd.thread_id = gt.id AND rd.account_id = ?
ORDER BY gm.created_at DESC, gt.id DESC`

type guardianThreadRow struct {
	groupThreadRow
	LastMessageBody string    `bun:"last_message_body"`
	LastVisibleAt   time.Time `bun:"last_visible_at"`
	StudentIDs      []int64   `bun:"student_ids,array"`
	UnreadCount     int       `bun:"unread_count"`
}

// ListThreadsForGuardian returns the guardian's group threads in the given
// schools for the given children. It runs in an administrative transaction:
// a family's children may attend several schools.
func (s *GroupThreadStore) ListThreadsForGuardian(ctx context.Context, accountID int64, studentIDs, tenantIDs []int64) ([]communication.PortalGuardianGroupThread, error) {
	if accountID <= 0 || len(studentIDs) == 0 || len(tenantIDs) == 0 {
		return []communication.PortalGuardianGroupThread{}, nil
	}
	db, _, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []guardianThreadRow
	if err := db.NewRaw(guardianGroupThreadListSQL, accountID, bun.List(studentIDs), bun.List(tenantIDs), accountID).
		Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("list guardian group threads: %w", err)
	}
	out := make([]communication.PortalGuardianGroupThread, 0, len(rows))
	for i := range rows {
		at := rows[i].LastVisibleAt
		out = append(out, communication.PortalGuardianGroupThread{
			Thread: *rows[i].groupThreadRow.value(), LastMessageBody: rows[i].LastMessageBody,
			LastMessageAt: &at, UnreadCount: rows[i].UnreadCount, StudentIDs: rows[i].StudentIDs,
		})
	}
	return out, nil
}

// guardianGroupUnreadSQL counts the guardian's threads with an unread
// message. Bind order: account, students, schools.
const guardianGroupUnreadSQL = `
SELECT COUNT(DISTINCT r.thread_id)
FROM users.parent_group_message_recipients AS r
LEFT JOIN users.parent_group_thread_reads AS rd ON rd.thread_id = r.thread_id AND rd.account_id = r.guardian_account_id
WHERE r.guardian_account_id = ? AND r.student_id IN (?) AND r.tenant_id IN (?)
	AND r.message_id > COALESCE(rd.last_read_message_id, 0)`

// CountUnreadThreadsForGuardian counts the guardian's group threads with an
// unread message, across the given schools.
func (s *GroupThreadStore) CountUnreadThreadsForGuardian(ctx context.Context, accountID int64, studentIDs, tenantIDs []int64) (int, error) {
	if accountID <= 0 || len(studentIDs) == 0 || len(tenantIDs) == 0 {
		return 0, nil
	}
	db, _, err := s.database(ctx)
	if err != nil {
		return 0, err
	}
	var count int
	if err := db.NewRaw(guardianGroupUnreadSQL, accountID, bun.List(studentIDs), bun.List(tenantIDs)).
		Scan(ctx, &count); err != nil {
		return 0, fmt.Errorf("count guardian group threads: %w", err)
	}
	return count, nil
}

// ListMessagesForGuardian returns the messages of one thread that reached the
// guardian for one of the given children, oldest first, with those children.
func (s *GroupThreadStore) ListMessagesForGuardian(ctx context.Context, threadID, accountID int64, studentIDs []int64) ([]communication.ParentGroupMessage, []int64, error) {
	if len(studentIDs) == 0 {
		return []communication.ParentGroupMessage{}, []int64{}, nil
	}
	db, tenantID, err := s.tenantDB(ctx, "list guardian group messages")
	if err != nil {
		return nil, nil, err
	}
	var rows []groupMessageRow
	if err := db.NewSelect().Model(&rows).
		Where("gm.thread_id = ? AND gm.tenant_id = ?", threadID, tenantID).
		Where(`EXISTS (SELECT 1 FROM users.parent_group_message_recipients AS r
			WHERE r.message_id = gm.id AND r.guardian_account_id = ? AND r.student_id IN (?))`,
			accountID, bun.List(studentIDs)).
		OrderExpr("gm.id ASC").
		Scan(ctx); err != nil {
		return nil, nil, fmt.Errorf("list guardian group messages: %w", err)
	}
	var children []int64
	if err := db.NewSelect().TableExpr("users.parent_group_message_recipients AS r").
		ColumnExpr("DISTINCT r.student_id").
		Where("r.thread_id = ? AND r.tenant_id = ? AND r.guardian_account_id = ? AND r.student_id IN (?)",
			threadID, tenantID, accountID, bun.List(studentIDs)).
		OrderExpr("r.student_id").
		Scan(ctx, &children); err != nil {
		return nil, nil, fmt.Errorf("list guardian group children: %w", err)
	}
	out := make([]communication.ParentGroupMessage, 0, len(rows))
	for i := range rows {
		out = append(out, rows[i].value())
	}
	return out, children, nil
}

// MarkGuardianRead advances the guardian's cursor to messageID and never
// back. It reports whether the cursor moved.
func (s *GroupThreadStore) MarkGuardianRead(ctx context.Context, threadID, accountID, messageID int64) (bool, error) {
	db, tenantID, err := s.tenantDB(ctx, "mark parent group thread read")
	if err != nil {
		return false, err
	}
	result, err := db.ExecContext(ctx, `
		INSERT INTO users.parent_group_thread_reads (tenant_id, thread_id, account_id, last_read_message_id)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (thread_id, account_id) DO UPDATE
			SET last_read_message_id = EXCLUDED.last_read_message_id, read_at = clock_timestamp()
			WHERE users.parent_group_thread_reads.last_read_message_id < EXCLUDED.last_read_message_id`,
		tenantID, threadID, accountID, messageID)
	if err != nil {
		return false, fmt.Errorf("mark parent group thread read: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("mark parent group thread read: %w", err)
	}
	return affected == 1, nil
}

// IsRecipient reports whether a message of the thread reached the guardian
// for the child. Only such a guardian may answer the thread for the child.
func (s *GroupThreadStore) IsRecipient(ctx context.Context, threadID, accountID, studentID int64) (bool, error) {
	db, tenantID, err := s.tenantDB(ctx, "check parent group thread recipient")
	if err != nil {
		return false, err
	}
	exists, err := db.NewSelect().TableExpr("users.parent_group_message_recipients AS r").
		Where("r.thread_id = ? AND r.tenant_id = ? AND r.guardian_account_id = ? AND r.student_id = ?",
			threadID, tenantID, accountID, studentID).
		Exists(ctx)
	if err != nil {
		return false, fmt.Errorf("check parent group thread recipient: %w", err)
	}
	return exists, nil
}
