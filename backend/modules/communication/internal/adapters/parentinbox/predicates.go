package parentinbox

import (
	"maps"
	"slices"

	"github.com/uptrace/bun/dialect/pgdialect"
)

// The unread predicates below are written out per message alias and per reader
// side instead of being formatted at runtime. Every fragment a query builder
// receives has to be a compile-time constant, so the architecture evaluator can
// read which tables and columns this projection touches; a formatted string is
// opaque to it. The two aliases are cm (the inbox's correlated count) and um
// (the aggregate badge query).

// counterpartUnread* is "a message from the OTHER party relative to the
// reader": a staff reader counts unread guardian-side activity, a guardian
// reader counts unread staff-side activity.
//
// A system event (request decision / withdrawal) carries sender_kind='system'
// and records the side that TRIGGERED it in event_actor_kind, so the
// counterpart side cannot be read from sender_kind for those rows. A staff
// confirm/reject is unread to the guardian; a parent withdrawal is unread to
// staff.
//
// request_created pills are excluded from EVERY unread number: a submitted
// change request is surfaced on the request-queue badge, never on the messages
// badge. The pill still shows in the timeline as a notice. IS DISTINCT FROM
// keeps plain messages (NULL event_type) and every other pill counted.
const (
	counterpartUnreadCMForStaff    = `((cm.sender_kind = 'guardian' OR (cm.sender_kind = 'system' AND cm.event_actor_kind = 'guardian')) AND cm.event_type IS DISTINCT FROM 'request_created')`
	counterpartUnreadCMForGuardian = `((cm.sender_kind = 'staff' OR (cm.sender_kind = 'system' AND cm.event_actor_kind = 'staff')) AND cm.event_type IS DISTINCT FROM 'request_created')`
	counterpartUnreadUMForStaff    = `((um.sender_kind = 'guardian' OR (um.sender_kind = 'system' AND um.event_actor_kind = 'guardian')) AND um.event_type IS DISTINCT FROM 'request_created')`
	counterpartUnreadUMForGuardian = `((um.sender_kind = 'staff' OR (um.sender_kind = 'system' AND um.event_actor_kind = 'staff')) AND um.event_type IS DISTINCT FROM 'request_created')`
)

// afterReadCursor* is the composite tie-break "this message is strictly after
// the reader's cursor", comparing (created_at, id) against (last_read_at,
// last_read_message_id) of the joined cursor row r. It is the correctness core
// of every unread number.
const (
	afterReadCursorCM = `(cm.created_at, cm.id) > (COALESCE(r.last_read_at, '1970-01-01'::timestamptz), COALESCE(r.last_read_message_id, 0))`
	afterReadCursorUM = `(um.created_at, um.id) > (COALESCE(r.last_read_at, '1970-01-01'::timestamptz), COALESCE(r.last_read_message_id, 0))`
)

// afterStaffHandledCursor* keeps only guardian activity not yet covered by a
// team reply. It supplements, never replaces, each account's own read cursor.
const (
	afterStaffHandledCursorCM = `(cm.created_at, cm.id) > (COALESCE(t.staff_handled_up_to_at, '1970-01-01'::timestamptz), COALESCE(t.staff_handled_up_to_message_id, 0))`
	afterStaffHandledCursorUM = `(um.created_at, um.id) > (COALESCE(t.staff_handled_up_to_at, '1970-01-01'::timestamptz), COALESCE(t.staff_handled_up_to_message_id, 0))`
)

// notReaderAuthored* excludes the reader's OWN plain messages from their unread
// set. Each carries a single ? bound to the reader's account id at the call
// site.
//
// This is what keeps a dual-role (staff+guardian) account from counting its own
// just-sent message as unread to itself: that account is its own counterpart.
// Enforcing it here rather than advancing the sender's cursor on send is why the
// append path does not move the cursor — a cursor leap to the just-sent message
// would also skip an earlier counterpart message that committed afterwards.
//
// System events are DELIBERATELY exempt: they carry the triggering side in
// event_actor_kind and are attributed by SIDE, not account, so counterpartUnread
// is their sole gate. Applying the account exclusion to them would cancel
// exactly the attribution that lights the other portal's badge.
const (
	notReaderAuthoredCM = `(cm.sender_kind = 'system' OR cm.sender_account_id <> ?)`
	notReaderAuthoredUM = `(um.sender_kind = 'system' OR um.sender_account_id <> ?)`
)

// unreadCountSubquery* is the inbox's per-thread unread column.
//
// cm.tenant_id = t.tenant_id is REQUIRED for index usability, not only for RLS.
// The only index on parent_messages leads with tenant_id, and the cross-tenant
// guardian queries run under an admin transaction where RLS injects no tenant
// predicate — a thread_id-only correlated filter would then seq-scan
// parent_messages once per thread row.
const (
	unreadCountPrefix = `(
		SELECT COUNT(*) FROM users.parent_messages cm
		WHERE cm.thread_id = t.id AND cm.tenant_id = t.tenant_id
		  AND `
	unreadCountSuffix = `
	) AS unread_count`

	unreadCountForStaff = unreadCountPrefix +
		counterpartUnreadCMForStaff + `
		  AND ` + afterReadCursorCM + `
		  AND ` + notReaderAuthoredCM + `
		  AND ` + afterStaffHandledCursorCM + unreadCountSuffix

	unreadCountForGuardian = unreadCountPrefix +
		counterpartUnreadCMForGuardian + `
		  AND ` + afterReadCursorCM + `
		  AND ` + notReaderAuthoredCM + unreadCountSuffix
)

// staffCursorPairsOn* match a read cursor against the (school, staff account)
// pairs the caller resolved. The two are the same test on a different cursor
// row: OnCursorRow reads the receipt lookup's own `r` row, OnReceiptRow the
// `sr` row of the inbox's correlated last-message subquery.
//
// Passing the pairs as two parallel arrays keeps the
// SQL a single constant no matter how many schools are in scope, and keeps the
// school correlated with its own staff: the cross-tenant guardian queries would
// otherwise accept a staff account of school A as proof that school B read the
// message. The staff rows belong to School Membership, so the projection is
// handed the resolved accounts rather than joining that owner's table.
const (
	staffCursorPairsOnCursorRow = `(t.tenant_id, r.account_id) IN (
		SELECT * FROM unnest(?::bigint[], ?::bigint[])
	)`
	staffCursorPairsOnReceiptRow = `(t.tenant_id, sr.account_id) IN (
		SELECT * FROM unnest(?::bigint[], ?::bigint[])
	)`
	noStaffCursor = `FALSE`
)

// staffCursorArgs flattens the (school -> staff accounts) map into the parallel
// arrays the pair predicate binds. An empty set yields no args, and the caller
// then uses noStaffCursor so nothing can match.
func staffCursorArgs(staffAccounts map[int64][]int64) ([]any, bool) {
	tenantIDs := make([]int64, 0, len(staffAccounts))
	accountIDs := make([]int64, 0, len(staffAccounts))
	for _, tenantID := range slices.Sorted(maps.Keys(staffAccounts)) {
		for _, accountID := range staffAccounts[tenantID] {
			tenantIDs = append(tenantIDs, tenantID)
			accountIDs = append(accountIDs, accountID)
		}
	}
	if len(tenantIDs) == 0 {
		return nil, false
	}
	return []any{pgdialect.Array(tenantIDs), pgdialect.Array(accountIDs)}, true
}

// lastMessageReadByStaff reports the parent-facing "OGS hat gelesen" flag on
// the thread's last message. It is only computed for guardian readers; the
// staff inbox never renders it.
const lastMessageReadByStaffPrefix = `(
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
		  AND `

const (
	lastMessageReadByStaffSuffix = `
	)
) AS last_message_read_by_staff`

	lastMessageReadByStaffWithStaff = lastMessageReadByStaffPrefix + staffCursorPairsOnReceiptRow + lastMessageReadByStaffSuffix
	lastMessageReadByStaffNever     = lastMessageReadByStaffPrefix + noStaffCursor + lastMessageReadByStaffSuffix
	lastMessageReadByStaffAbsent    = `FALSE AS last_message_read_by_staff`
)

// threadHasMessages keeps conversations that were opened (get-or-create) but
// never written to out of the lists: an empty thread must not appear until its
// first message. It checks message existence directly rather than the
// denormalized last_message_at, so it holds even if that field drifts.
const threadHasMessages = `EXISTS (
	SELECT 1 FROM users.parent_messages hm
	WHERE hm.thread_id = t.id AND hm.tenant_id = t.tenant_id
)`

// guardianStillLinked keeps a guardian-facing thread visible only while the
// guardian still has a live link to the thread's student in that school AND
// that link still grants parent_portal.access.
//
// Without it, a guardian unlinked from child A but still linked to child B at
// the same school keeps that school in their scope, so child A's row and unread
// badge keep rendering: the display-only sg LEFT JOIN does not gate visibility.
// The containment check mirrors the open-thread permission gate, so a guardian
// who is primary for child A but pickup-only for child B does not see child B's
// thread metadata. Guardian-facing queries only; staff see a child's threads
// regardless of guardian link.
const guardianStillLinked = `EXISTS (
	SELECT 1 FROM users.students_guardians sg_link
	JOIN users.guardian_profiles gp_link ON gp_link.id = sg_link.guardian_profile_id
	WHERE sg_link.student_id = t.student_id
	  AND gp_link.account_id = t.guardian_account_id
	  AND gp_link.tenant_id = t.tenant_id
	  AND sg_link.permissions @> '{"parent_portal.access": true}'::jsonb
)`

// guardianUnreadExists keeps or drops whole threads for the staff inbox's
// unread filter. It mirrors the inbox unread_count column exactly, and carries a
// single ? (notReaderAuthored) bound at the call site.
const guardianUnreadExists = `EXISTS (
	SELECT 1 FROM users.parent_messages um
	WHERE um.thread_id = t.id AND um.tenant_id = t.tenant_id
	  AND ` + counterpartUnreadUMForStaff + `
	  AND ` + afterReadCursorUM + `
	  AND ` + notReaderAuthoredUM + `
	  AND ` + afterStaffHandledCursorUM + `
)`

// withTenant applies the defense-in-depth tenant_id filter that complements
// RLS. A zero tenant leaves the query untouched, which is what the
// administrative and cross-tenant guardian paths rely on.
//
// It mirrors the filter the repositories this projection replaces applied, and
// keeps the same shape as its sibling in parentpostgres.
func withTenant[Q interface{ Where(string, ...any) Q }](query Q, alias string, tenantID int64) Q {
	if tenantID <= 0 {
		return query
	}
	return query.Where(`"`+alias+`".tenant_id = ?`, tenantID)
}
