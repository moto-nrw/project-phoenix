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

// MarkReadUpTo upserts the reader's cursor to the composite (readAt,
// readMessageID) instead of NOW(), never moving it backward. The cursor advances
// only when the new composite is greater than the stored one (timestamp first,
// then id), so a stale list snapshot can never roll the cursor back and a tied
// message keeps its tie-breaker. See the interface doc for why a NOW() cursor
// would swallow a message that committed after the caller's list snapshot.
func (r *ParentMessageReadRepository) MarkReadUpTo(ctx context.Context, tenantID, threadID, accountID int64, readAt time.Time, readMessageID int64) error {
	row := &users.ParentMessageRead{
		ThreadID:          threadID,
		AccountID:         accountID,
		LastReadAt:        readAt,
		LastReadMessageID: readMessageID,
	}
	row.SetTenantID(tenantID)

	// Advance the (last_read_at, last_read_message_id) pair atomically: compare the
	// incoming composite against the stored one and take the incoming only when it
	// is strictly greater. Advancing the two columns with independent GREATEST()
	// could mix a newer timestamp with an older id (or vice versa) and corrupt the
	// cursor.
	const advance = `(EXCLUDED.last_read_at, EXCLUDED.last_read_message_id) > ` +
		`(parent_message_reads.last_read_at, parent_message_reads.last_read_message_id)`
	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(row).
		ModelTableExpr("users.parent_message_reads").
		On("CONFLICT (thread_id, account_id) DO UPDATE").
		Set("last_read_at = CASE WHEN " + advance + " THEN EXCLUDED.last_read_at ELSE parent_message_reads.last_read_at END").
		Set("last_read_message_id = CASE WHEN " + advance + " THEN EXCLUDED.last_read_message_id ELSE parent_message_reads.last_read_message_id END").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "mark parent message thread read up to", Err: err}
	}
	return nil
}

// counterpartUnread builds the SQL boolean for "a message from the OTHER party
// relative to the reader", on the given message alias: a staff reader counts
// unread guardian messages, a guardian reader counts unread staff messages.
func counterpartUnread(alias string, staffReader bool) string {
	if staffReader {
		return fmt.Sprintf(`%s.sender_kind = 'guardian'`, alias)
	}
	return fmt.Sprintf(`%s.sender_kind = 'staff'`, alias)
}

// inboxSelect builds the InboxThread projection. staffReader switches the
// "unread" side: staff readers count unread guardian-side activity, guardian
// readers count unread staff-side activity (see counterpartUnread).
func inboxSelect(q *bun.SelectQuery, accountID int64, staffReader bool) *bun.SelectQuery {
	unreadSub := fmt.Sprintf(`(
		SELECT COUNT(*) FROM users.parent_messages cm
		WHERE cm.thread_id = t.id
		  AND %s
		  AND (cm.created_at, cm.id) > (COALESCE(r.last_read_at, '1970-01-01'::timestamptz), COALESCE(r.last_read_message_id, 0))
	) AS unread_count`, counterpartUnread("cm", staffReader))

	return q.
		TableExpr("users.parent_message_threads AS t").
		ColumnExpr("t.id AS thread_id").
		ColumnExpr("t.tenant_id AS tenant_id").
		ColumnExpr("t.student_id AS student_id").
		ColumnExpr("btrim(COALESCE(pn.first_name,'') || ' ' || COALESCE(pn.last_name,'')) AS student_name").
		ColumnExpr("s.school_class AS school_class").
		ColumnExpr("COALESCE(sch.name,'') AS school_name").
		ColumnExpr("s.group_id AS group_id").
		ColumnExpr("COALESCE(g.name,'') AS group_name").
		ColumnExpr("t.guardian_account_id AS guardian_account_id").
		ColumnExpr("btrim(COALESCE(gp.first_name,'') || ' ' || COALESCE(gp.last_name,'')) AS guardian_name").
		ColumnExpr("COALESCE(sg.relationship_type,'') AS relationship_type").
		ColumnExpr("t.last_message_at AS last_message_at").
		ColumnExpr("COALESCE(t.last_sender_kind,'') AS last_sender_kind").
		// last_message_body is denormalized onto the thread (maintained wherever
		// last_message_at is set), so the inbox preview no longer needs a
		// correlated subquery that re-scans parent_messages per thread row.
		ColumnExpr("COALESCE(t.last_message_body,'') AS last_message_body").
		ColumnExpr(unreadSub).
		Join("JOIN users.students AS s ON s.id = t.student_id").
		Join("JOIN users.persons AS pn ON pn.id = s.person_id AND pn.deleted_at IS NULL").
		Join("LEFT JOIN platform.schools AS sch ON sch.id = t.tenant_id").
		Join("LEFT JOIN education.groups AS g ON g.id = s.group_id").
		// gp.tenant_id = t.tenant_id is REQUIRED: guardian_profiles is
		// UNIQUE(tenant_id, account_id), so a guardian with children at two OGS has
		// one profile row per tenant. Joining on account_id alone would match every
		// tenant's row and duplicate each inbox thread once per tenant.
		Join("LEFT JOIN users.guardian_profiles AS gp ON gp.account_id = t.guardian_account_id AND gp.tenant_id = t.tenant_id").
		Join("LEFT JOIN users.students_guardians AS sg ON sg.guardian_profile_id = gp.id AND sg.student_id = t.student_id").
		Join("LEFT JOIN users.parent_message_reads AS r ON r.thread_id = t.id AND r.account_id = ? AND r.tenant_id = t.tenant_id", accountID).
		OrderExpr("t.last_message_at DESC NULLS LAST")
}

// threadHasMessages keeps conversations that were opened (get-or-create) but
// never written to out of the inbox / thread lists: an empty thread must not
// appear until its first message. Checks message existence directly rather
// than the denormalized last_message_at so it holds even if that field drifts.
const threadHasMessages = `EXISTS (
	SELECT 1 FROM users.parent_messages hm WHERE hm.thread_id = t.id
)`

// guardianStillLinked keeps a guardian-facing thread visible only while the
// guardian still has a LIVE students_guardians link to the thread's student in
// that tenant AND that link still grants parent_portal.access. Without it, a
// guardian unlinked from child A but still linked to child B at the same school
// keeps tenant A in their tenant set, so child A's thread row (name, class,
// group, last-message preview) and unread badge keep rendering even though the
// relationship is gone — the inbox `sg` LEFT JOIN is display-only and does not
// gate visibility.
//
// The parent_portal.access containment check mirrors the open-thread gate
// (GetChildConversation → resolveOwnedChild → resolvePermittedChild with
// GuardianPermissionPortalAccess): a guardian who is primary for child A but
// pickup_only/custom-without-access for child B at the same school must NOT see
// child B's thread metadata in their list, even though child A pulls that tenant
// into scope. Role presets store the permission as JSON `true`
// (StudentGuardianPermissionSet), so jsonb containment is the exact, NULL-safe,
// cast-free equivalent of authorize.StudentGuardianHasPermission. See
// .claude/rules/guardian-parent-permissions.md.
//
// Guardian-facing queries only: staff still see a child's threads regardless of
// guardian link.
const guardianStillLinked = `EXISTS (
	SELECT 1 FROM users.students_guardians sg_link
	JOIN users.guardian_profiles gp_link ON gp_link.id = sg_link.guardian_profile_id
	WHERE sg_link.student_id = t.student_id
	  AND gp_link.account_id = t.guardian_account_id
	  AND gp_link.tenant_id = t.tenant_id
	  AND sg_link.permissions @> '{"parent_portal.access": true}'::jsonb
)`

// guardianUnreadExists is the EXISTS predicate for "unread guardian-side
// activity" for the staff reader (used by the inbox onlyUnread filter and the
// badge). Mirrors the inbox unread_count column: guardian messages plus the
// guardian's own system events (e.g. a withdrawn request).
var guardianUnreadExists = fmt.Sprintf(`EXISTS (
	SELECT 1 FROM users.parent_messages um
	WHERE um.thread_id = t.id
	  AND %s
	  AND (um.created_at, um.id) > (COALESCE(r.last_read_at, '1970-01-01'::timestamptz), COALESCE(r.last_read_message_id, 0))
)`, counterpartUnread("um", true))

// staffUnreadExists is the guardian-reader counterpart of guardianUnreadExists:
// "unread staff-side activity" (staff messages plus staff-triggered system
// events such as a confirmed/rejected request). Drives the parent-portal unread
// badge count.
var staffUnreadExists = fmt.Sprintf(`EXISTS (
	SELECT 1 FROM users.parent_messages um
	WHERE um.thread_id = t.id
	  AND %s
	  AND (um.created_at, um.id) > (COALESCE(r.last_read_at, '1970-01-01'::timestamptz), COALESCE(r.last_read_message_id, 0))
)`, counterpartUnread("um", false))

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
	query := inboxSelect(base.GetDB(ctx, r.db).NewSelect(), accountID, true)
	query = applyStaffScope(query, allStudents, groupIDs)
	query = query.Where(threadHasMessages)
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

// ListThreadsForStudent returns the threads about a single child (staff view),
// newest activity first; unread counts guardian-side activity. The caller must
// have already authorized read access to the child — this only adds the
// student_id filter so the staff student-detail card stops fetching the whole
// tenant inbox and filtering client-side.
func (r *ParentMessageReadRepository) ListThreadsForStudent(ctx context.Context, accountID, studentID int64) ([]*users.InboxThread, error) {
	var rows []*users.InboxThread
	query := inboxSelect(base.GetDB(ctx, r.db).NewSelect(), accountID, true).
		Where("t.student_id = ?", studentID).
		Where(threadHasMessages)
	if where, val, ok := base.TenantWhere(ctx, "t"); ok {
		query = query.Where(where, val)
	}
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list parent message threads for student", Err: err}
	}
	return rows, nil
}

// ListThreadsForGuardianStudent returns the guardian's own threads about ONE of
// their children in the current tenant; unread counts staff-side activity. The
// per-child detail page uses this so it stops fetching the guardian's whole
// cross-tenant inbox just to render one child's conversation. Runs under the
// child's tenant tx (the caller resolves ownership first).
func (r *ParentMessageReadRepository) ListThreadsForGuardianStudent(ctx context.Context, accountID, studentID int64) ([]*users.InboxThread, error) {
	var rows []*users.InboxThread
	query := inboxSelect(base.GetDB(ctx, r.db).NewSelect(), accountID, false).
		Where("t.guardian_account_id = ?", accountID).
		Where("t.student_id = ?", studentID).
		Where(threadHasMessages).
		Where(guardianStillLinked)
	if where, val, ok := base.TenantWhere(ctx, "t"); ok {
		query = query.Where(where, val)
	}
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list guardian threads for student", Err: err}
	}
	return rows, nil
}

// ListThreadsForGuardianTenants returns the guardian's threads across the given
// tenants in one query, newest-activity first. Cross-tenant: it filters on the
// explicit tenant set instead of the per-request tenant scope, so it must run
// under WithAdminTx (RLS bypassed). The guardian_account_id predicate keeps the
// result to the guardian's OWN threads; tenantIDs gates it to the schools the
// guardian currently has children at (preserving the per-tenant ownership gate
// the old one-tx-per-tenant loop provided).
func (r *ParentMessageReadRepository) ListThreadsForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64) ([]*users.InboxThread, error) {
	if len(tenantIDs) == 0 {
		return []*users.InboxThread{}, nil
	}
	var rows []*users.InboxThread
	query := inboxSelect(base.GetDB(ctx, r.db).NewSelect(), accountID, false).
		Where("t.guardian_account_id = ?", accountID).
		Where("t.tenant_id IN (?)", bun.List(tenantIDs)).
		Where(threadHasMessages).
		Where(guardianStillLinked)
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list guardian threads cross-tenant", Err: err}
	}
	return rows, nil
}

// UnreadThreadCountForGuardianTenants counts the guardian's unread threads
// across the given tenants in one query — the parent-portal sidebar badge
// source. Cross-tenant: run under WithAdminTx. See ListThreadsForGuardianTenants
// for the ownership/scoping rationale.
func (r *ParentMessageReadRepository) UnreadThreadCountForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64) (int, error) {
	if len(tenantIDs) == 0 {
		return 0, nil
	}
	count, err := base.GetDB(ctx, r.db).NewSelect().
		TableExpr("users.parent_message_threads AS t").
		// Mirror the guardian thread LIST (ListThreadsForGuardianTenants → inboxSelect),
		// which joins persons with deleted_at IS NULL and so hides an offboarded
		// child's thread. Without this join the count would still include it,
		// leaving the portal badge stuck at "1 unread" with no thread to open.
		Join("JOIN users.students AS s ON s.id = t.student_id").
		Join("JOIN users.persons AS pn ON pn.id = s.person_id AND pn.deleted_at IS NULL").
		Join("LEFT JOIN users.parent_message_reads AS r ON r.thread_id = t.id AND r.account_id = ? AND r.tenant_id = t.tenant_id", accountID).
		Where("t.guardian_account_id = ?", accountID).
		Where("t.tenant_id IN (?)", bun.List(tenantIDs)).
		Where(guardianStillLinked).
		Where(staffUnreadExists).
		Count(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "count unread guardian threads cross-tenant", Err: err}
	}
	return count, nil
}

// FindThreadHeader returns only the chat-window header fields (student/guardian
// names + relationship) with a light join, or nil when the thread does not
// exist / is out of tenant. It avoids the inbox projection's two correlated
// COUNT(*) subqueries over parent_messages, which the header never uses.
func (r *ParentMessageReadRepository) FindThreadHeader(ctx context.Context, threadID int64) (*users.ThreadHeader, error) {
	var rows []*users.ThreadHeader
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr("users.parent_message_threads AS t").
		ColumnExpr("btrim(COALESCE(pn.first_name,'') || ' ' || COALESCE(pn.last_name,'')) AS student_name").
		ColumnExpr("btrim(COALESCE(gp.first_name,'') || ' ' || COALESCE(gp.last_name,'')) AS guardian_name").
		ColumnExpr("COALESCE(sg.relationship_type,'') AS relationship_type").
		Join("JOIN users.students AS s ON s.id = t.student_id").
		Join("JOIN users.persons AS pn ON pn.id = s.person_id AND pn.deleted_at IS NULL").
		// gp.tenant_id = t.tenant_id is REQUIRED: guardian_profiles is
		// UNIQUE(tenant_id, account_id), so joining on account_id alone would
		// duplicate the row once per tenant for a guardian with children at two OGS.
		Join("LEFT JOIN users.guardian_profiles AS gp ON gp.account_id = t.guardian_account_id AND gp.tenant_id = t.tenant_id").
		Join("LEFT JOIN users.students_guardians AS sg ON sg.guardian_profile_id = gp.id AND sg.student_id = t.student_id").
		Where("t.id = ?", threadID)
	if where, val, ok := base.TenantWhere(ctx, "t"); ok {
		query = query.Where(where, val)
	}
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find thread header", Err: err}
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

// LatestReadAtByOther returns the newest read cursor in the thread among
// accounts other than excludeAccountID. Used by the parent portal for the
// "OGS hat gelesen" trust indicator.
func (r *ParentMessageReadRepository) LatestReadAtByOther(ctx context.Context, threadID, excludeAccountID int64) (*time.Time, error) {
	var out struct {
		LastReadAt *time.Time `bun:"last_read_at"`
	}
	// The "OGS hat gelesen" receipt must reflect a STAFF read, never another
	// guardian's. A relationship-based exclusion ("drop every guardian of this
	// student") is wrong in BOTH directions: it still counts an unrelated parent
	// of a DIFFERENT child whose cursor the migration seeded for every active
	// tenant account (false "read"), and it wrongly drops a dual-role teacher who
	// is ALSO this child's guardian — whose read is a legitimate staff read. So
	// gate POSITIVELY on staff membership: count a cursor only when its account
	// belongs to a (non-deleted) staff member at the thread's tenant. The querying
	// account is still excluded so the guardian's own read never satisfies "the
	// OGS read it".
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr("users.parent_message_reads AS r").
		Join("JOIN users.parent_message_threads AS t ON t.id = r.thread_id").
		ColumnExpr("MAX(r.last_read_at) AS last_read_at").
		Where("r.thread_id = ?", threadID).
		Where("r.account_id <> ?", excludeAccountID).
		Where(`EXISTS (
			SELECT 1
			FROM users.staff st
			JOIN users.persons p ON p.id = st.person_id
			WHERE p.account_id = r.account_id
			  AND st.tenant_id = t.tenant_id
			  AND st.deleted_at IS NULL
			  AND p.deleted_at IS NULL
		)`)
	if where, val, ok := base.TenantWhere(ctx, "r"); ok {
		query = query.Where(where, val)
	}
	if err := query.Scan(ctx, &out); err != nil {
		return nil, &modelBase.DatabaseError{Op: "latest parent message read cursor", Err: err}
	}
	return out.LastReadAt, nil
}

// UnreadThreadCountForStaff counts threads with an unread guardian message
// for the staff reader, within their visible student scope.
func (r *ParentMessageReadRepository) UnreadThreadCountForStaff(ctx context.Context, accountID int64, allStudents bool, groupIDs []int64) (int, error) {
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr("users.parent_message_threads AS t").
		Join("JOIN users.students AS s ON s.id = t.student_id").
		// Exclude soft-deleted students so an offboarded child's unread thread
		// can't leave a permanent, unclearable badge: the inbox LIST joins
		// persons with deleted_at IS NULL (inboxSelect) and hides such threads,
		// so the COUNT must match or it inflates past every openable thread.
		Join("JOIN users.persons AS pn ON pn.id = s.person_id AND pn.deleted_at IS NULL").
		Join("LEFT JOIN users.parent_message_reads AS r ON r.thread_id = t.id AND r.account_id = ? AND r.tenant_id = t.tenant_id", accountID).
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
