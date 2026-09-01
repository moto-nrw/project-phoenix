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
//
// Returns whether the cursor actually ADVANCED (true on a first read or a real
// forward move, false on a stale/no-op write). The read-receipt SSE push gates on
// this so a refetch that marks nothing new read does not fire an event — which is
// what stops the receipt event from ping-ponging with the refetch it triggers on
// the other side. The monotonic guard is now the ON CONFLICT ... WHERE clause
// (no-advance simply touches no row), so RowsAffected is exactly "advanced".
func (r *ParentMessageReadRepository) MarkReadUpTo(ctx context.Context, tenantID, threadID, accountID int64, readAt time.Time, readMessageID int64) (bool, error) {
	row := &users.ParentMessageRead{
		ThreadID:          threadID,
		AccountID:         accountID,
		LastReadAt:        readAt,
		LastReadMessageID: readMessageID,
	}
	row.SetTenantID(tenantID)

	// Advance the (last_read_at, last_read_message_id) pair atomically: compare the
	// incoming composite against the stored one and take the incoming only when it
	// is strictly greater. The strictly-greater test lives in the ON CONFLICT DO
	// UPDATE ... WHERE so a non-advance touches NO row (RowsAffected == 0) — that is
	// what makes the returned bool an exact "did the cursor move". A CASE in SET
	// would always write the row (RowsAffected == 1) and lose that signal. Comparing
	// the composite as a tuple (not two independent GREATEST()) keeps a newer
	// timestamp from mixing with an older id and corrupting the cursor.
	const advance = `(EXCLUDED.last_read_at, EXCLUDED.last_read_message_id) > ` +
		`(parent_message_reads.last_read_at, parent_message_reads.last_read_message_id)`
	res, err := base.GetDB(ctx, r.db).NewInsert().
		Model(row).
		ModelTableExpr("users.parent_message_reads").
		On("CONFLICT (thread_id, account_id) DO UPDATE").
		Set("last_read_at = EXCLUDED.last_read_at").
		Set("last_read_message_id = EXCLUDED.last_read_message_id").
		Where(advance).
		Exec(ctx)
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "mark parent message thread read up to", Err: base.TranslateNotFound(err)}
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "mark parent message thread read up to", Err: base.TranslateNotFound(err)}
	}
	return affected > 0, nil
}

// MarkStaffHandledUpTo advances the shared staff boundary atomically. The
// composite guard keeps stale or concurrent replies from moving it backward.
func (r *ParentMessageReadRepository) MarkStaffHandledUpTo(ctx context.Context, tenantID, threadID int64, handledAt time.Time, handledMessageID int64) error {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*users.ParentMessageThread)(nil)).
		ModelTableExpr(`users.parent_message_threads AS "thread"`).
		Set("staff_handled_up_to_at = ?", handledAt).
		Set("staff_handled_up_to_message_id = ?", handledMessageID).
		Where(`"thread".id = ?`, threadID).
		Where(`"thread".tenant_id = ?`, tenantID).
		Where(`("thread".staff_handled_up_to_at IS NULL OR
			("thread".staff_handled_up_to_at, "thread".staff_handled_up_to_message_id) < (?, ?))`, handledAt, handledMessageID)
	query = base.WithTenantFilter(ctx, query, "thread")
	if _, err := query.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "mark parent message thread handled for staff", Err: base.TranslateNotFound(err)}
	}
	return nil
}

// counterpartUnread builds the SQL boolean for "a message from the OTHER party
// relative to the reader", on the given message alias: a staff reader counts
// unread guardian-side activity, a guardian reader counts unread staff-side
// activity.
//
// A system event (request decision / withdrawal) carries sender_kind='system'
// and records the side that TRIGGERED it in event_actor_kind, so the counterpart
// side cannot be read from sender_kind for those rows — it must come from
// event_actor_kind. A staff confirm/reject (event_actor_kind='staff') is unread
// to the guardian; a parent withdrawal (event_actor_kind='guardian') is unread to
// staff. Plain messages match on sender_kind directly. Centralizing both cases
// here keeps every unread number (inbox COUNT column, sidebar badges, unread
// EXISTS filter) consistent — see the callers of this helper.
func counterpartUnread(alias string, staffReader bool) string {
	side := "staff"
	if staffReader {
		side = "guardian"
	}
	// request_created pills are excluded from EVERY unread number here: a
	// submitted change request is surfaced as a count on the Änderungsanfragen
	// queue badge (its own actionable signal), never as an unread chat message on
	// the Nachrichten badge (#1803 — kills the double-signal). The pill still
	// lives in the thread timeline as a non-interactive notice. IS DISTINCT FROM
	// keeps plain messages (NULL event_type) and every OTHER pill counted; only
	// this one event_type drops out. Baking it into the shared predicate is what
	// keeps the aggregate badge count, the per-thread unread_count column, and the
	// unread-EXISTS filter from ever disagreeing. For guardian readers it is a
	// no-op — a guardian-side pill is already not their counterpart — so the
	// exclusion only affects the staff side, by design.
	return fmt.Sprintf(
		`((%[1]s.sender_kind = '%[2]s' OR (%[1]s.sender_kind = 'system' AND %[1]s.event_actor_kind = '%[2]s')) AND %[1]s.event_type IS DISTINCT FROM 'request_created')`,
		alias, side,
	)
}

// afterReadCursor builds the load-bearing composite tie-break "message <alias>
// is strictly after the reader's read cursor", comparing (created_at, id)
// against (last_read_at, last_read_message_id) of the joined cursor row `r`.
// This fragment is the correctness core of every unread number in the feature
// (inbox COUNT column + both unread-EXISTS badges), so it lives in ONE place —
// a fix here can't land in one copy and silently skip the others, which would
// make the inbox list count and the sidebar badge diverge.
func afterReadCursor(alias string) string {
	return fmt.Sprintf(
		`(%[1]s.created_at, %[1]s.id) > (COALESCE(r.last_read_at, '1970-01-01'::timestamptz), COALESCE(r.last_read_message_id, 0))`,
		alias,
	)
}

// afterStaffHandledCursor keeps only guardian activity not yet covered by a
// team reply. It supplements, rather than replaces, each account's read cursor.
func afterStaffHandledCursor(alias string) string {
	return fmt.Sprintf(
		`(%[1]s.created_at, %[1]s.id) > (COALESCE(t.staff_handled_up_to_at, '1970-01-01'::timestamptz), COALESCE(t.staff_handled_up_to_message_id, 0))`,
		alias,
	)
}

// notReaderAuthored excludes the reader's OWN plain messages from their unread
// set, keyed on sender_account_id (a real column). It is the third leg of every
// unread predicate, and carries a single `?` bound to the reader's account id at
// the call site.
//
// This is what keeps a dual-role (staff+guardian) account from counting its own
// just-sent message as unread to itself: that account is the counterpart of
// itself, so its guardian-sent message matches a staff reader's
// counterpartUnread('guardian') (and vice versa). The model contract is exactly
// "unread = a message after the cursor the reader did NOT send" (see
// models/users/parent_message_read.go). Enforcing it here — rather than advancing
// the sender's read cursor on send — is why parentmessaging.AppendMessage no
// longer moves the cursor: a cursor leap to the just-sent message also skips an
// earlier counterpart message that committed after the send (lower created_at,
// later commit), silently marking an unseen message read.
//
// System events (request decisions / withdrawals) are DELIBERATELY exempt from
// the account exclusion: they carry the triggering side in event_actor_kind and
// are attributed by SIDE, not account — counterpartUnread already decides which
// portal they are unread to. appendSystemEvent stores the ACTOR in
// sender_account_id, so for a dual-role account a confirm/reject it triggers as
// staff (or a withdrawal as guardian) would match its own account and be filtered
// back out of the OPPOSITE side's unread set — silently cancelling exactly the
// event_actor_kind attribution that exists to light the other portal's badge. So
// the self-exclusion applies to plain messages only; for system events
// counterpartUnread is the sole, side-correct gate.
func notReaderAuthored(alias string) string {
	return fmt.Sprintf(`(%[1]s.sender_kind = 'system' OR %[1]s.sender_account_id <> ?)`, alias)
}

// inboxSelect builds the InboxThread projection. staffReader switches the
// "unread" side: staff readers count unread guardian-side activity, guardian
// readers count unread staff-side activity (see counterpartUnread).
func inboxSelect(q *bun.SelectQuery, accountID int64, staffReader bool) *bun.SelectQuery {
	// cm.tenant_id = t.tenant_id is REQUIRED for index usability, not just RLS. The
	// only index on parent_messages leads with tenant_id (tenant_id, thread_id,
	// created_at); the cross-tenant guardian queries (ListThreadsForGuardianTenants)
	// run this under WithAdminTx where RLS is bypassed and injects no tenant
	// predicate, so a thread_id-only correlated filter cannot use the index and
	// seq-scans parent_messages per thread row. Binding the leading column makes it
	// an index scan. Redundant-but-correct under RLS-scoped staff queries.
	unreadPredicates := fmt.Sprintf(`
		  AND %s
		  AND %s
		  AND %s`, counterpartUnread("cm", staffReader), afterReadCursor("cm"), notReaderAuthored("cm"))
	if staffReader {
		unreadPredicates += fmt.Sprintf("\n\t\t  AND %s", afterStaffHandledCursor("cm"))
	}
	unreadSub := fmt.Sprintf(`(
		SELECT COUNT(*) FROM users.parent_messages cm
		WHERE cm.thread_id = t.id AND cm.tenant_id = t.tenant_id%s
	) AS unread_count`, unreadPredicates)
	lastMessageReadByStaff := "FALSE AS last_message_read_by_staff"
	if !staffReader {
		lastMessageReadByStaff = `(
			t.last_sender_kind = 'guardian'
			AND EXISTS (
				SELECT 1
				FROM users.parent_message_reads sr
				WHERE sr.thread_id = t.id
				  AND sr.tenant_id = t.tenant_id
				  AND sr.account_id <> t.guardian_account_id
				  AND (
					sr.last_read_at > t.last_message_at
					OR (sr.last_read_at = t.last_message_at AND sr.last_read_message_id >= t.last_message_id)
				  )
				  AND EXISTS (
					SELECT 1
					FROM users.staff st
					JOIN users.persons p ON p.id = st.person_id
					WHERE p.account_id = sr.account_id
					  AND st.tenant_id = t.tenant_id
					  AND st.deleted_at IS NULL
					  AND p.deleted_at IS NULL
				  )
			)
		) AS last_message_read_by_staff`
	}

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
		// Structured fields of the last message (joined as lm on last_message_id
		// below), so the LOCALIZED parents portal renders a request title or a
		// decision/withdrawal system-event preview from fields instead of the
		// German last_message_body. Empty for a fresh thread (no last message) and
		// for plain messages, where the body is already language-neutral. The
		// German-only staff inbox ignores these.
		ColumnExpr("COALESCE(lm.kind,'') AS last_message_kind").
		ColumnExpr("COALESCE(lm.event_type,'') AS last_event_type").
		ColumnExpr("COALESCE(lm.request_type,'') AS last_request_type").
		ColumnExpr("COALESCE(lm.request_status,'') AS last_request_status").
		ColumnExpr("lm.payload AS last_message_payload").
		ColumnExpr(lastMessageReadByStaff).
		// accountID binds the notReaderAuthored `?` in unreadSub (cm.sender_account_id
		// <> ?). bun renders args in SQL-fragment order, so this select-list arg
		// precedes the read-cursor join's account-id arg below; both are the same id.
		ColumnExpr(unreadSub, accountID).
		Join("JOIN users.students AS s ON s.id = t.student_id").
		Join("JOIN users.persons AS pn ON pn.id = s.person_id AND pn.deleted_at IS NULL").
		Where("s.status <> ?", users.StudentStatusAlumnus).
		Join("LEFT JOIN platform.schools AS sch ON sch.id = t.tenant_id").
		Join("LEFT JOIN education.groups AS g ON g.id = s.group_id").
		// gp.tenant_id = t.tenant_id is REQUIRED: guardian_profiles is
		// UNIQUE(tenant_id, account_id), so a guardian with children at two OGS has
		// one profile row per tenant. Joining on account_id alone would match every
		// tenant's row and duplicate each inbox thread once per tenant.
		Join("LEFT JOIN users.guardian_profiles AS gp ON gp.account_id = t.guardian_account_id AND gp.tenant_id = t.tenant_id").
		Join("LEFT JOIN users.students_guardians AS sg ON sg.guardian_profile_id = gp.id AND sg.student_id = t.student_id").
		Join("LEFT JOIN users.parent_message_reads AS r ON r.thread_id = t.id AND r.account_id = ? AND r.tenant_id = t.tenant_id", accountID).
		// The thread's last message, for the structured preview columns above. PK
		// lookup (one row per thread), so it stays an index scan; tenant_id = t.tenant_id
		// mirrors the other correlated reads (RLS-correct and index-leading under the
		// cross-tenant guardian path). NULL last_message_id (fresh thread) -> no row,
		// columns COALESCE to ''.
		Join("LEFT JOIN users.parent_messages AS lm ON lm.id = t.last_message_id AND lm.tenant_id = t.tenant_id").
		OrderExpr("t.last_message_at DESC NULLS LAST")
}

// threadHasMessages keeps conversations that were opened (get-or-create) but
// never written to out of the inbox / thread lists: an empty thread must not
// appear until its first message. Checks message existence directly rather
// than the denormalized last_message_at so it holds even if that field drifts.
// hm.tenant_id = t.tenant_id binds the leading index column so this EXISTS uses
// the (tenant_id, thread_id, ...) index even under WithAdminTx (cross-tenant
// guardian queries), where RLS injects no tenant predicate — see unreadSub.
const threadHasMessages = `EXISTS (
	SELECT 1 FROM users.parent_messages hm
	WHERE hm.thread_id = t.id AND hm.tenant_id = t.tenant_id
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

// guardianUnreadExists is the EXISTS predicate for "this thread has at least one
// unread guardian-side message" for the staff reader, used by the inbox onlyUnread
// FILTER (which keeps or drops whole threads). The sidebar badge no longer uses it
// — it counts unread messages via unreadMessageCountSelect. Mirrors the inbox
// unread_count column: guardian messages plus the guardian's own system events
// (e.g. a withdrawn request). It carries a single `?` (notReaderAuthored) bound to
// the reader's account id at the call site.
var guardianUnreadExists = fmt.Sprintf(`EXISTS (
	SELECT 1 FROM users.parent_messages um
	WHERE um.thread_id = t.id AND um.tenant_id = t.tenant_id
	  AND %s
	  AND %s
	  AND %s
	  AND %s
)`, counterpartUnread("um", true), afterReadCursor("um"), notReaderAuthored("um"), afterStaffHandledCursor("um"))

// unreadMessageCountSelect builds a query whose ROWS are the unread MESSAGES for
// the given reader: counterpart-authored messages strictly after the reader's
// read cursor, excluding the reader's own. It is the aggregate twin of
// inboxSelect's unread_count COLUMN — same three predicates (counterpartUnread,
// afterReadCursor, notReaderAuthored) — but spanning many threads, so the sidebar
// badges count unread messages and match the per-thread pills instead of counting
// unread threads. staffReader switches the counterpart side (staff count guardian
// messages, guardians count staff messages). Callers add the scope/ownership
// filters (staff scope, or guardian + tenant set) and Count().
//
// parent_messages is the base table; the threads/persons joins reuse inboxSelect's
// soft-delete, alumnus, and tenant-index discipline: pn.deleted_at IS NULL hides
// an offboarded child's messages, s.status <> alumnus hides a graduated child's
// (both so a badge can't outlive every openable thread — the inboxes filter
// these threads out, and a count that didn't would leave a nonzero badge no
// portal can clear, #405 review), and um.tenant_id = t.tenant_id binds the
// leading index column so the guardian cross-tenant variant under WithAdminTx
// still uses the (tenant_id, thread_id, created_at) index. The r LEFT JOIN is
// unique per (thread, account), so no row fans out and COUNT(*) is an exact
// message count.
func unreadMessageCountSelect(q *bun.SelectQuery, accountID int64, staffReader bool) *bun.SelectQuery {
	query := q.
		TableExpr("users.parent_messages AS um").
		Join("JOIN users.parent_message_threads AS t ON t.id = um.thread_id AND t.tenant_id = um.tenant_id").
		Join("JOIN users.students AS s ON s.id = t.student_id").
		Join("JOIN users.persons AS pn ON pn.id = s.person_id AND pn.deleted_at IS NULL").
		Join("LEFT JOIN users.parent_message_reads AS r ON r.thread_id = t.id AND r.account_id = ? AND r.tenant_id = t.tenant_id", accountID).
		Where("s.status <> ?", users.StudentStatusAlumnus).
		Where(counterpartUnread("um", staffReader)).
		Where(afterReadCursor("um")).
		Where(notReaderAuthored("um"), accountID)
	if staffReader {
		query = query.Where(afterStaffHandledCursor("um"))
	}
	return query
}

// applyStaffScope narrows a thread query to the students a staff member may
// read: everything for admins and verified staff (#2329), nothing for any
// other caller (impossible filter, so the result is empty).
func applyStaffScope(q *bun.SelectQuery, allStudents bool) *bun.SelectQuery {
	if allStudents {
		return q
	}
	return q.Where("1 = 0")
}

// ListInboxForStaff returns the staff member's readable threads, newest
// activity first. onlyUnread keeps only threads with an unread guardian message.
func (r *ParentMessageReadRepository) ListInboxForStaff(ctx context.Context, accountID int64, allStudents bool, onlyUnread bool) ([]*users.InboxThread, error) {
	var rows []*users.InboxThread
	query := inboxSelect(base.GetDB(ctx, r.db).NewSelect(), accountID, true)
	query = applyStaffScope(query, allStudents)
	query = query.Where(threadHasMessages)
	query = base.WithTenantFilter(ctx, query, "t")
	if onlyUnread {
		query = query.Where(guardianUnreadExists, accountID)
	}
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list parent message inbox", Err: base.TranslateNotFound(err)}
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
	query = base.WithTenantFilter(ctx, query, "t")
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list parent message threads for student", Err: base.TranslateNotFound(err)}
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
	query = base.WithTenantFilter(ctx, query, "t")
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list guardian threads for student", Err: base.TranslateNotFound(err)}
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
		return nil, &modelBase.DatabaseError{Op: "list guardian threads cross-tenant", Err: base.TranslateNotFound(err)}
	}
	return rows, nil
}

// UnreadMessageCountForGuardianTenants counts the guardian's unread staff
// MESSAGES across the given tenants in one query — the parent-portal sidebar
// badge source. It counts messages (not threads) so the badge matches the staff
// side and the per-thread pills. Cross-tenant: run under WithAdminTx. See
// ListThreadsForGuardianTenants for the ownership/scoping rationale; the
// persons deleted_at IS NULL join and the alumnus filter inside
// unreadMessageCountSelect hide an offboarded or graduated child's messages so
// the badge can't outlive its openable thread.
func (r *ParentMessageReadRepository) UnreadMessageCountForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64) (int, error) {
	if len(tenantIDs) == 0 {
		return 0, nil
	}
	count, err := unreadMessageCountSelect(base.GetDB(ctx, r.db).NewSelect(), accountID, false).
		Where("t.guardian_account_id = ?", accountID).
		Where("t.tenant_id IN (?)", bun.List(tenantIDs)).
		Where(guardianStillLinked).
		Count(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "count unread guardian messages cross-tenant", Err: base.TranslateNotFound(err)}
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
	query = base.WithTenantFilter(ctx, query, "t")
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find thread header", Err: base.TranslateNotFound(err)}
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

// LatestReadCursorByOther returns the furthest read cursor (by the composite
// (last_read_at, last_read_message_id)) in the thread among STAFF accounts other
// than excludeAccountID, or nil when none exists. Used by the parent portal for
// the "OGS hat gelesen" trust indicator. Returning the composite (not just the
// MAX timestamp) lets the receipt compare on the same tie-break the unread
// predicates use, so a tied higher-id message is not stamped read prematurely.
func (r *ParentMessageReadRepository) LatestReadCursorByOther(ctx context.Context, threadID, excludeAccountID int64) (*users.ReadCursor, error) {
	var rows []users.ReadCursor
	// The "OGS hat gelesen" receipt must reflect a STAFF read, never another
	// guardian's. A relationship-based exclusion ("drop every guardian of this
	// student") is wrong in BOTH directions: it still counts an unrelated parent
	// of a DIFFERENT child whose cursor the migration seeded for every active
	// tenant account (false "read"), and it wrongly drops a dual-role teacher who
	// is ALSO this child's guardian — whose read is a legitimate staff read. So
	// gate POSITIVELY on staff membership: count a cursor only when its account
	// belongs to a (non-deleted) staff member at the thread's tenant. The querying
	// account is still excluded so the guardian's own read never satisfies "the
	// OGS read it". ORDER BY the composite DESC + LIMIT 1 returns the single
	// furthest staff cursor (a message read by ANY staff member counts as read).
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr("users.parent_message_reads AS r").
		Join("JOIN users.parent_message_threads AS t ON t.id = r.thread_id").
		ColumnExpr("r.last_read_at AS last_read_at").
		ColumnExpr("r.last_read_message_id AS last_read_message_id").
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
		)`).
		OrderExpr("r.last_read_at DESC").
		OrderExpr("r.last_read_message_id DESC").
		Limit(1)
	query = base.WithTenantFilter(ctx, query, "r")
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "latest parent message read cursor", Err: base.TranslateNotFound(err)}
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

// GuardianReadCursor returns the read cursor of the thread's guardian account, or
// nil when the guardian has not read anything yet. Drives the staff-facing "von
// den Eltern gelesen" receipt. Unlike LatestReadCursorByOther (which aggregates
// over every staff cursor and excludes the querying account), a thread has exactly
// ONE guardian account (t.guardian_account_id), so this matches the single read
// row for that account directly — no staff-membership gate, no aggregation. The
// composite (last_read_at, last_read_message_id) is returned so the receipt
// compares on the same tie-break the unread predicates use.
func (r *ParentMessageReadRepository) GuardianReadCursor(ctx context.Context, threadID int64) (*users.ReadCursor, error) {
	var rows []users.ReadCursor
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr("users.parent_message_reads AS r").
		Join("JOIN users.parent_message_threads AS t ON t.id = r.thread_id").
		ColumnExpr("r.last_read_at AS last_read_at").
		ColumnExpr("r.last_read_message_id AS last_read_message_id").
		Where("r.thread_id = ?", threadID).
		Where("r.account_id = t.guardian_account_id").
		Limit(1)
	query = base.WithTenantFilter(ctx, query, "r")
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "guardian parent message read cursor", Err: base.TranslateNotFound(err)}
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

// UnreadMessageCountForStaff counts unread guardian MESSAGES for the staff
// reader, within their visible student scope — the sidebar badge source. It
// counts messages (not threads) so the badge matches the per-thread unread pills
// in the inbox: a thread with three unread guardian messages contributes 3, not 1.
func (r *ParentMessageReadRepository) UnreadMessageCountForStaff(ctx context.Context, accountID int64, allStudents bool) (int, error) {
	query := unreadMessageCountSelect(base.GetDB(ctx, r.db).NewSelect(), accountID, true)
	query = applyStaffScope(query, allStudents)
	query = base.WithTenantFilter(ctx, query, "t")
	count, err := query.Count(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "count unread parent messages for staff", Err: base.TranslateNotFound(err)}
	}
	return count, nil
}
