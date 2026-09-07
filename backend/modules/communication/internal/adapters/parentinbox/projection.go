// Package parentinbox implements the tenant-safe parent-conversation
// projection: the inbox and thread lists, the unread badges, the chat header,
// and the two read receipts.
//
// It is a read-only projection because every one of those answers joins
// Communication's parent_message* tables with People Directory's student,
// person, and guardian rows. It never writes; the cursor and message writes
// stay with their owner in parentpostgres.
package parentinbox

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"
	"github.com/uptrace/bun"
)

// Database resolves the caller's ambient transaction plus the tenant the
// statement must be scoped to. A zero tenant means administrative or
// cross-tenant work, where the query relies on its explicit school filter.
type Database func(context.Context) (bun.IDB, int64, error)

// studentStatusAlumnus mirrors the People Directory status that hides a
// graduated child. It is a literal here because a projection reads another
// owner's rows without importing that owner's package.
const studentStatusAlumnus = "alumnus"

// Projection answers every parent-conversation read that spans owners.
type Projection struct{ database Database }

// New builds the projection over the ambient transaction runtime.
func New(database Database) *Projection {
	if database == nil {
		panic("communication parent inbox: database runtime is required")
	}
	return &Projection{database: database}
}

type threadHeaderRow struct {
	StudentName      string `bun:"student_name"`
	GuardianName     string `bun:"guardian_name"`
	RelationshipType string `bun:"relationship_type"`
}

type readCursorRow struct {
	LastReadAt        time.Time `bun:"last_read_at"`
	LastReadMessageID int64     `bun:"last_read_message_id"`
}

type inboxRow struct {
	ThreadID               int64          `bun:"thread_id"`
	TenantID               int64          `bun:"tenant_id"`
	StudentID              int64          `bun:"student_id"`
	StudentName            string         `bun:"student_name"`
	SchoolClass            string         `bun:"school_class"`
	GroupID                *int64         `bun:"group_id"`
	GuardianAccountID      int64          `bun:"guardian_account_id"`
	GuardianName           string         `bun:"guardian_name"`
	RelationshipType       string         `bun:"relationship_type"`
	LastMessageAt          *time.Time     `bun:"last_message_at"`
	LastSenderKind         string         `bun:"last_sender_kind"`
	LastMessageBody        string         `bun:"last_message_body"`
	LastMessageKind        string         `bun:"last_message_kind"`
	LastEventType          string         `bun:"last_event_type"`
	LastRequestType        string         `bun:"last_request_type"`
	LastRequestStatus      string         `bun:"last_request_status"`
	LastMessagePayload     map[string]any `bun:"last_message_payload"`
	LastMessageReadByStaff bool           `bun:"last_message_read_by_staff"`
	UnreadCount            int            `bun:"unread_count"`
}

func inboxValues(rows []inboxRow) []*domain.ParentInboxThread {
	threads := make([]*domain.ParentInboxThread, 0, len(rows))
	for _, row := range rows {
		threads = append(threads, &domain.ParentInboxThread{
			ThreadID: row.ThreadID, TenantID: row.TenantID, StudentID: row.StudentID,
			StudentName: row.StudentName, SchoolClass: row.SchoolClass, GroupID: row.GroupID,
			GuardianAccountID: row.GuardianAccountID, GuardianName: row.GuardianName,
			RelationshipType: row.RelationshipType, LastMessageAt: row.LastMessageAt,
			LastSenderKind: row.LastSenderKind, LastMessageBody: row.LastMessageBody,
			LastMessageKind: row.LastMessageKind, LastEventType: row.LastEventType,
			LastRequestType: row.LastRequestType, LastRequestStatus: row.LastRequestStatus,
			LastMessagePayload:     row.LastMessagePayload,
			LastMessageReadByStaff: row.LastMessageReadByStaff,
			UnreadCount:            row.UnreadCount,
		})
	}
	return threads
}

// withReadByStaffColumn adds the parent-facing "OGS hat gelesen" flag for the
// thread's last message. Each branch passes its SQL literally: the architecture
// evaluator reads the tables a query touches out of the constant it receives,
// so a fragment handed over through a variable would be opaque to it.
func withReadByStaffColumn(q *bun.SelectQuery, staffReader bool, staffAccounts map[int64][]int64) *bun.SelectQuery {
	if staffReader {
		// The staff inbox never renders the flag.
		return q.ColumnExpr(lastMessageReadByStaffAbsent)
	}
	args, ok := staffCursorArgs(staffAccounts)
	if !ok {
		// No staff account in scope: no cursor can prove the OGS read it.
		return q.ColumnExpr(lastMessageReadByStaffNever)
	}
	return q.ColumnExpr(lastMessageReadByStaffWithStaff, args...)
}

// withUnreadCountColumn adds the per-thread unread count. accountID binds the
// notReaderAuthored placeholder inside the subquery.
func withUnreadCountColumn(q *bun.SelectQuery, accountID int64, staffReader bool) *bun.SelectQuery {
	if staffReader {
		return q.ColumnExpr(unreadCountForStaff, accountID)
	}
	return q.ColumnExpr(unreadCountForGuardian, accountID)
}

// inboxSelect builds the inbox projection. staffReader switches the unread
// side: staff readers count unread guardian-side activity, guardian readers
// count unread staff-side activity, and only guardian readers compute the
// "OGS hat gelesen" flag on the last message.
func inboxSelect(q *bun.SelectQuery, accountID int64, staffReader bool, staffAccounts map[int64][]int64) *bun.SelectQuery {
	q = withReadByStaffColumn(q, staffReader, staffAccounts)
	q = withUnreadCountColumn(q, accountID, staffReader)
	return q.
		TableExpr("users.parent_message_threads AS t").
		ColumnExpr("t.id AS thread_id").
		ColumnExpr("t.tenant_id AS tenant_id").
		ColumnExpr("t.student_id AS student_id").
		ColumnExpr("btrim(COALESCE(pn.first_name,'') || ' ' || COALESCE(pn.last_name,'')) AS student_name").
		ColumnExpr("s.school_class AS school_class").
		ColumnExpr("s.group_id AS group_id").
		ColumnExpr("t.guardian_account_id AS guardian_account_id").
		ColumnExpr("btrim(COALESCE(gp.first_name,'') || ' ' || COALESCE(gp.last_name,'')) AS guardian_name").
		ColumnExpr("COALESCE(sg.relationship_type,'') AS relationship_type").
		ColumnExpr("t.last_message_at AS last_message_at").
		ColumnExpr("COALESCE(t.last_sender_kind,'') AS last_sender_kind").
		// last_message_body is denormalized onto the thread, so the preview no
		// longer needs a correlated subquery re-scanning parent_messages per row.
		ColumnExpr("COALESCE(t.last_message_body,'') AS last_message_body").
		// Structured fields of the last message, so the LOCALIZED parents portal
		// renders a request title or a decision preview from fields instead of the
		// German body. Empty for a fresh thread and for plain messages.
		ColumnExpr("COALESCE(lm.kind,'') AS last_message_kind").
		ColumnExpr("COALESCE(lm.event_type,'') AS last_event_type").
		ColumnExpr("COALESCE(lm.request_type,'') AS last_request_type").
		ColumnExpr("COALESCE(lm.request_status,'') AS last_request_status").
		ColumnExpr("lm.payload AS last_message_payload").
		Join("JOIN users.students AS s ON s.id = t.student_id").
		Join("JOIN users.persons AS pn ON pn.id = s.person_id AND pn.deleted_at IS NULL").
		Where("s.status <> ?", studentStatusAlumnus).
		// gp.tenant_id = t.tenant_id is REQUIRED: guardian_profiles is
		// UNIQUE(tenant_id, account_id), so a guardian with children at two schools
		// has one row per school. Joining on account_id alone would duplicate each
		// inbox thread once per school.
		Join("LEFT JOIN users.guardian_profiles AS gp ON gp.account_id = t.guardian_account_id AND gp.tenant_id = t.tenant_id").
		Join("LEFT JOIN users.students_guardians AS sg ON sg.guardian_profile_id = gp.id AND sg.student_id = t.student_id").
		Join("LEFT JOIN users.parent_message_reads AS r ON r.thread_id = t.id AND r.account_id = ? AND r.tenant_id = t.tenant_id", accountID).
		// The thread's last message, for the structured preview columns above. A
		// primary-key lookup, so it stays an index scan; tenant_id = t.tenant_id
		// mirrors the other correlated reads.
		Join("LEFT JOIN users.parent_messages AS lm ON lm.id = t.last_message_id AND lm.tenant_id = t.tenant_id").
		OrderExpr("t.last_message_at DESC NULLS LAST")
}

// unreadMessageCountSelect builds a query whose ROWS are the reader's unread
// MESSAGES. It is the aggregate twin of the inbox unread_count column — same
// predicates — but spanning many threads, so the sidebar badges count unread
// messages and match the per-thread pills instead of counting threads.
//
// The person and alumnus filters mirror the lists: an offboarded or graduated
// child's messages are hidden, so a badge cannot outlive every openable thread.
// The r LEFT JOIN is unique per (thread, account), so no row fans out and
// COUNT(*) is an exact message count.
func unreadMessageCountSelect(q *bun.SelectQuery, accountID int64, staffReader bool) *bun.SelectQuery {
	query := q.
		TableExpr("users.parent_messages AS um").
		Join("JOIN users.parent_message_threads AS t ON t.id = um.thread_id AND t.tenant_id = um.tenant_id").
		Join("JOIN users.students AS s ON s.id = t.student_id").
		Join("JOIN users.persons AS pn ON pn.id = s.person_id AND pn.deleted_at IS NULL").
		Join("LEFT JOIN users.parent_message_reads AS r ON r.thread_id = t.id AND r.account_id = ? AND r.tenant_id = t.tenant_id", accountID).
		Where("s.status <> ?", studentStatusAlumnus).
		Where(afterReadCursorUM).
		Where(notReaderAuthoredUM, accountID)
	if staffReader {
		return query.Where(counterpartUnreadUMForStaff).Where(afterStaffHandledCursorUM)
	}
	return query.Where(counterpartUnreadUMForGuardian)
}

// applyStaffScope narrows a thread query to the students a staff member may
// read: everything for admins and verified staff, nothing for any other caller
// (an impossible filter, so the result is empty).
func applyStaffScope(q *bun.SelectQuery, allStudents bool) *bun.SelectQuery {
	if allStudents {
		return q
	}
	return q.Where("1 = 0")
}

// ListInboxForStaff returns the staff member's readable threads, newest
// activity first. onlyUnread keeps only threads with unread guardian activity.
func (p *Projection) ListInboxForStaff(ctx context.Context, accountID int64, allStudents, onlyUnread bool) ([]*domain.ParentInboxThread, error) {
	db, tenantID, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []inboxRow
	query := inboxSelect(db.NewSelect(), accountID, true, nil)
	query = applyStaffScope(query, allStudents)
	query = query.Where(threadHasMessages)
	query = withTenant(query, "t", tenantID)
	if onlyUnread {
		query = query.Where(guardianUnreadExists, accountID)
	}
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("list parent message inbox: %w", err)
	}
	return inboxValues(rows), nil
}

// ListThreadsForStudent returns the staff view of one child's threads, newest
// activity first. The caller must have authorized read access already; this
// only adds the student filter, so the student detail card stops fetching the
// whole school inbox and filtering client-side.
func (p *Projection) ListThreadsForStudent(ctx context.Context, accountID, studentID int64) ([]*domain.ParentInboxThread, error) {
	db, tenantID, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []inboxRow
	query := inboxSelect(db.NewSelect(), accountID, true, nil).
		Where("t.student_id = ?", studentID).
		Where(threadHasMessages)
	query = withTenant(query, "t", tenantID)
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("list parent message threads for student: %w", err)
	}
	return inboxValues(rows), nil
}

// ListThreadsForGuardianStudent returns the guardian's own threads about ONE of
// their children in the current school. staffAccounts maps each school to the
// login accounts of its live staff, resolved by the composition root through
// the School Membership owner.
func (p *Projection) ListThreadsForGuardianStudent(ctx context.Context, accountID, studentID int64, staffAccounts map[int64][]int64) ([]*domain.ParentInboxThread, error) {
	db, tenantID, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []inboxRow
	query := inboxSelect(db.NewSelect(), accountID, false, staffAccounts).
		Where("t.guardian_account_id = ?", accountID).
		Where("t.student_id = ?", studentID).
		Where(threadHasMessages).
		Where(guardianStillLinked)
	query = withTenant(query, "t", tenantID)
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("list guardian threads for student: %w", err)
	}
	return inboxValues(rows), nil
}

// ListThreadsForGuardianTenants returns the guardian's threads across the given
// schools in one query. Cross-tenant: it filters on the explicit school set
// instead of the per-request scope, so it runs under an admin transaction. The
// guardian predicate keeps the result to the guardian's own threads, and the
// school set preserves the per-school ownership gate the old one-transaction-
// per-school loop provided.
func (p *Projection) ListThreadsForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64, staffAccounts map[int64][]int64) ([]*domain.ParentInboxThread, error) {
	if len(tenantIDs) == 0 {
		return []*domain.ParentInboxThread{}, nil
	}
	db, _, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []inboxRow
	query := inboxSelect(db.NewSelect(), accountID, false, staffAccounts).
		Where("t.guardian_account_id = ?", accountID).
		Where("t.tenant_id IN (?)", bun.List(tenantIDs)).
		Where(threadHasMessages).
		Where(guardianStillLinked)
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("list guardian threads cross-tenant: %w", err)
	}
	return inboxValues(rows), nil
}

// UnreadMessageCountForStaff counts unread guardian MESSAGES within the staff
// reader's visible scope. It counts messages, not threads, so the sidebar badge
// matches the per-thread pills: a thread with three unread messages adds 3.
func (p *Projection) UnreadMessageCountForStaff(ctx context.Context, accountID int64, allStudents bool) (int, error) {
	db, tenantID, err := p.database(ctx)
	if err != nil {
		return 0, err
	}
	query := unreadMessageCountSelect(db.NewSelect(), accountID, true)
	query = applyStaffScope(query, allStudents)
	query = withTenant(query, "t", tenantID)
	count, err := query.Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("count unread parent messages for staff: %w", err)
	}
	return count, nil
}

// UnreadMessageCountForGuardianTenants counts the guardian's unread staff
// MESSAGES across the given schools in one query — the parent-portal badge.
// Cross-tenant: it runs under an admin transaction.
func (p *Projection) UnreadMessageCountForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64) (int, error) {
	if len(tenantIDs) == 0 {
		return 0, nil
	}
	db, _, err := p.database(ctx)
	if err != nil {
		return 0, err
	}
	count, err := unreadMessageCountSelect(db.NewSelect(), accountID, false).
		Where("t.guardian_account_id = ?", accountID).
		Where("t.tenant_id IN (?)", bun.List(tenantIDs)).
		Where(guardianStillLinked).
		Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("count unread guardian messages cross-tenant: %w", err)
	}
	return count, nil
}

// FindThreadHeader returns only the chat-window header fields with a light
// join, or nil when the thread does not exist or is out of scope. It avoids the
// inbox projection's correlated COUNT subqueries, which the header never uses.
func (p *Projection) FindThreadHeader(ctx context.Context, threadID int64) (*domain.ParentThreadHeader, error) {
	db, tenantID, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []threadHeaderRow
	query := db.NewSelect().
		TableExpr("users.parent_message_threads AS t").
		ColumnExpr("btrim(COALESCE(pn.first_name,'') || ' ' || COALESCE(pn.last_name,'')) AS student_name").
		ColumnExpr("btrim(COALESCE(gp.first_name,'') || ' ' || COALESCE(gp.last_name,'')) AS guardian_name").
		ColumnExpr("COALESCE(sg.relationship_type,'') AS relationship_type").
		Join("JOIN users.students AS s ON s.id = t.student_id").
		Join("JOIN users.persons AS pn ON pn.id = s.person_id AND pn.deleted_at IS NULL").
		// gp.tenant_id = t.tenant_id is REQUIRED: joining on account_id alone
		// duplicates the row once per school for a guardian with two schools.
		Join("LEFT JOIN users.guardian_profiles AS gp ON gp.account_id = t.guardian_account_id AND gp.tenant_id = t.tenant_id").
		Join("LEFT JOIN users.students_guardians AS sg ON sg.guardian_profile_id = gp.id AND sg.student_id = t.student_id").
		Where("t.id = ?", threadID)
	query = withTenant(query, "t", tenantID)
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("find thread header: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &domain.ParentThreadHeader{
		StudentName: rows[0].StudentName, GuardianName: rows[0].GuardianName,
		RelationshipType: rows[0].RelationshipType,
	}, nil
}

// LatestReadCursorByOtherStaff returns the furthest read cursor in the thread
// among STAFF accounts other than excludeAccountID, or nil when none exists. It
// drives the parent-facing "OGS hat gelesen" indicator.
//
// The staff gate is POSITIVE on purpose. Excluding "every guardian of this
// student" instead is wrong in both directions: it still counts an unrelated
// parent whose cursor a migration seeded (a false "read"), and it wrongly drops
// a dual-role teacher who is also this child's guardian, whose read IS a staff
// read. The querying account stays excluded so a guardian's own read never
// satisfies "the OGS read it". Returning the composite lets the receipt compare
// on the same tie-break the unread predicates use.
func (p *Projection) LatestReadCursorByOtherStaff(ctx context.Context, threadID, excludeAccountID int64, staffAccounts map[int64][]int64) (*domain.ParentReadCursor, error) {
	db, tenantID, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []readCursorRow
	query := db.NewSelect().
		TableExpr("users.parent_message_reads AS r").
		Join("JOIN users.parent_message_threads AS t ON t.id = r.thread_id").
		ColumnExpr("r.last_read_at AS last_read_at").
		ColumnExpr("r.last_read_message_id AS last_read_message_id").
		Where("r.thread_id = ?", threadID).
		Where("r.account_id <> ?", excludeAccountID).
		OrderExpr("r.last_read_at DESC").
		OrderExpr("r.last_read_message_id DESC").
		Limit(1)
	if args, ok := staffCursorArgs(staffAccounts); ok {
		query = query.Where(staffCursorPairsForReads, args...)
	} else {
		query = query.Where(noStaffCursor)
	}
	query = withTenant(query, "r", tenantID)
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("latest parent message read cursor: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &domain.ParentReadCursor{LastReadAt: rows[0].LastReadAt, LastReadMessageID: rows[0].LastReadMessageID}, nil
}

// GuardianReadCursor returns the read cursor of the thread's guardian account,
// or nil when the guardian has not read anything yet. It drives the staff-facing
// "von den Eltern gelesen" receipt. A thread has exactly one guardian account,
// so this matches that single row directly — no staff gate, no aggregation.
func (p *Projection) GuardianReadCursor(ctx context.Context, threadID int64) (*domain.ParentReadCursor, error) {
	db, tenantID, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []readCursorRow
	query := db.NewSelect().
		TableExpr("users.parent_message_reads AS r").
		Join("JOIN users.parent_message_threads AS t ON t.id = r.thread_id").
		ColumnExpr("r.last_read_at AS last_read_at").
		ColumnExpr("r.last_read_message_id AS last_read_message_id").
		Where("r.thread_id = ?", threadID).
		Where("r.account_id = t.guardian_account_id").
		Limit(1)
	query = withTenant(query, "r", tenantID)
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("guardian parent message read cursor: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &domain.ParentReadCursor{LastReadAt: rows[0].LastReadAt, LastReadMessageID: rows[0].LastReadMessageID}, nil
}
