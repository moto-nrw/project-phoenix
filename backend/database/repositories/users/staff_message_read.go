package users

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// StaffMessageReadRepository owns the read cursors and every projection derived
// from them (inbox rows, unread badge, recipient picker).
type StaffMessageReadRepository struct {
	db *bun.DB
	// staffAccounts resolves the login accounts of the school's live staff.
	// School Membership owns those rows, so the relation that used to be a
	// join is injected as a lookup; without one the repository fails closed.
	staffAccounts StaffAccountsFunc
	// identity holds the Identity & Access owner queries: the global account
	// switch (#2720), the school mapping and the role classification (#2721).
	identity StaffMessageIdentity
}

// StaffAccountsFunc returns the login accounts of the live staff members of
// the tenant in ctx. It is a plain function type so this package does not have
// to depend on the School Membership owner to state what it needs.
type StaffAccountsFunc func(ctx context.Context) ([]int64, error)

// StaffMessageIdentity are the Identity & Access owner queries the staff
// messaging reads filter through instead of naming auth tables.
type StaffMessageIdentity struct {
	// ActiveAccounts selects the ids of globally active platform accounts.
	ActiveAccounts ActiveAccountQuery
	// ActiveMemberships selects (account_id, tenant_id) of ACTIVE mappings.
	ActiveMemberships ActiveMembershipQuery
	// RoleClasses classifies the roles accounts hold at a school.
	RoleClasses SchoolRoleClassQuery
}

// NewStaffMessageReadRepository wires a fresh repository.
//
// Composite-key table, so this does NOT embed base.Repository[T]: the generic
// helpers assume a single autoincrement id. It stays a plain struct with
// explicit queries, mirroring ParentMessageReadRepository.
//
// The "is a colleague at this school" relation needs the staff rows School
// Membership owns and the account facts Identity & Access owns, so the
// caller injects both.
func NewStaffMessageReadRepository(db *bun.DB, staffAccounts StaffAccountsFunc, identity StaffMessageIdentity) users.StaffMessageReadRepository {
	return &StaffMessageReadRepository{db: db, staffAccounts: staffAccounts, identity: identity}
}

// resolveStaffAccounts fails closed: without a resolver nobody is a colleague,
// so a misconfigured graph cannot widen who may be written to.
func (r *StaffMessageReadRepository) resolveStaffAccounts(ctx context.Context) ([]int64, error) {
	if r.staffAccounts == nil {
		return nil, &modelBase.DatabaseError{Op: "resolve staff accounts", Err: errors.New("staff account resolver is required")}
	}
	return r.staffAccounts(ctx)
}

// colleagueQuery applies the "this account belongs to a colleague at this
// school" relation, shared verbatim by ListMessageableStaff and
// IsMessageableStaff.
//
// It lives in ONE place because the two MUST agree: the picker decides what a
// user is offered, the predicate decides what the API accepts, and any drift
// between them is an authorization hole reachable by hand-crafting a request.
//
// Four facts, because each answers a different question and the account
// lifecycle has two independent switches:
//   - the ACTIVE school mapping says "may act at this school". Identity &
//     Access owns it, so the owner's membership query is joined as "at";
//   - users.persons is NOT enough: it also holds children and guests, who can
//     carry an account and an active tenant mapping;
//   - staff membership says "colleague at this school" — the caller passes the
//     accounts of the school's live staff, resolved through the School
//     Membership owner, and staffAccountFilter turns that into the predicate
//     the users.staff join used to be;
//   - auth.accounts.active is the GLOBAL switch. Account management
//     deactivates an account there WITHOUT touching the school mapping, so a
//     per-tenant check alone still lets a globally disabled account be
//     addressed and keep writing; the owner's active-account query covers it.
//
// It fails closed like resolveStaffAccounts: a graph without the owner
// queries addresses nobody.
func (r *StaffMessageReadRepository) colleagueQuery(ctx context.Context, query *bun.SelectQuery) (*bun.SelectQuery, error) {
	if r.identity.ActiveAccounts == nil || r.identity.ActiveMemberships == nil {
		return nil, &modelBase.DatabaseError{Op: "resolve active accounts", Err: errors.New("active account and membership queries are required")}
	}
	staffAccountIDs, err := r.resolveStaffAccounts(ctx)
	if err != nil {
		return nil, err
	}
	query = query.
		Join(`JOIN (?) AS "at" ON at.account_id = person.account_id AND at.tenant_id = person.tenant_id`, r.identity.ActiveMemberships(ctx)).
		Where(`person.deleted_at IS NULL`).
		Where(`at.tenant_id = ?`, tenant.FromContext(ctx)).
		Where(`at.account_id IN (?)`, r.identity.ActiveAccounts(ctx))
	return staffAccountFilter(query, staffAccountIDs), nil
}

// staffAccountFilter narrows the colleague relation to the accounts of the
// school's live staff. An empty set matches nothing, which is what the
// dropped INNER JOIN did.
func staffAccountFilter(query *bun.SelectQuery, staffAccountIDs []int64) *bun.SelectQuery {
	if len(staffAccountIDs) == 0 {
		return query.Where("1 = 0")
	}
	return query.Where(`at.account_id IN (?)`, bun.List(staffAccountIDs))
}

// The unread predicate is the correctness core of every unread number in this
// feature: "message <alias> is strictly after the reader's cursor AND the
// reader did not write it".
//
// The three constants below are the SAME predicate over three message aliases
// (m for the sidebar badge, cm for the inbox's per-thread unread_count column,
// um for the onlyUnread filter). They are spelled out because the query
// analyser attributes only constant SQL fragments to their tables; keep the
// three bodies textually identical apart from the alias, so a fix cannot land
// in one copy and silently skip the others — which is how an inbox count and a
// sidebar badge start disagreeing.
//
// The comparison is a TUPLE, not two independent tests: clock_timestamp() can
// stamp two messages with the same created_at, and the message list breaks
// those ties by id. Comparing the pair keeps a newer timestamp from mixing with
// an older id and skipping a message out of the unread set. The `?` binds the
// reader's account id.
const (
	unreadPredicateM = `(m.created_at, m.id) > (COALESCE(r.last_read_at, '1970-01-01'::timestamptz), COALESCE(r.last_read_message_id, 0))
		 AND m.sender_account_id <> ?`
	unreadPredicateCM = `(cm.created_at, cm.id) > (COALESCE(r.last_read_at, '1970-01-01'::timestamptz), COALESCE(r.last_read_message_id, 0))
		 AND cm.sender_account_id <> ?`
	unreadPredicateUM = `(um.created_at, um.id) > (COALESCE(r.last_read_at, '1970-01-01'::timestamptz), COALESCE(r.last_read_message_id, 0))
		 AND um.sender_account_id <> ?`
)

// MarkReadUpTo advances the account's cursor in one thread to the supplied
// composite.
//
// The strictly-greater test lives in the ON CONFLICT DO UPDATE ... WHERE so a
// non-advance touches NO row. A CASE in SET would always write and lose that
// property, letting an out-of-order request drag the cursor backwards and
// resurrect already-read messages in the badge.
func (r *StaffMessageReadRepository) MarkReadUpTo(ctx context.Context, threadID, accountID int64, at time.Time, messageID int64) error {
	row := &users.StaffMessageRead{
		ThreadID:          threadID,
		AccountID:         accountID,
		LastReadAt:        at,
		LastReadMessageID: messageID,
	}
	base.EnsureTenantID(ctx, row)

	const advance = `(EXCLUDED.last_read_at, EXCLUDED.last_read_message_id) > ` +
		`(staff_message_reads.last_read_at, staff_message_reads.last_read_message_id)`

	if _, err := base.GetDB(ctx, r.db).NewInsert().
		Model(row).
		ModelTableExpr("users.staff_message_reads").
		On("CONFLICT (thread_id, account_id) DO UPDATE").
		Set("last_read_at = EXCLUDED.last_read_at").
		Set("last_read_message_id = EXCLUDED.last_read_message_id").
		Where(advance).
		Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "mark staff message thread read up to", Err: base.TranslateNotFound(err)}
	}
	return nil
}

// UnreadCount is the account's total unread messages across every conversation
// it takes part in — the sidebar badge.
func (r *StaffMessageReadRepository) UnreadCount(ctx context.Context, accountID int64) (int, error) {
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`users.staff_messages AS "m"`).
		ColumnExpr(`COUNT(*)`).
		Join(`JOIN users.staff_message_participants AS "p"
			ON p.thread_id = m.thread_id AND p.account_id = ?`, accountID).
		Join(`LEFT JOIN users.staff_message_reads AS "r"
			ON r.thread_id = m.thread_id AND r.account_id = ?`, accountID).
		Where(unreadPredicateM, accountID)

	query = base.WithTenantFilter(ctx, query, "m")

	count := 0
	if err := query.Scan(ctx, &count); err != nil {
		return 0, &modelBase.DatabaseError{Op: "count unread staff messages", Err: base.TranslateNotFound(err)}
	}
	return count, nil
}

// ListInbox projects the account's conversations, newest activity first, with
// the counterpart and the per-thread unread count resolved.
//
// The counterpart is resolved per viewer: the same thread row renders as "Anna"
// for Ben and as "Ben" for Anna, so the join picks the participant that is NOT
// the viewer. Threads without any message are skipped — a get-or-create that
// was never followed by a send must not clutter the inbox.
func (r *StaffMessageReadRepository) ListInbox(ctx context.Context, accountID int64, onlyUnread bool) ([]*users.StaffInboxThread, error) {
	const unreadSub = `(
		SELECT COUNT(*)
		FROM users.staff_messages cm
		WHERE cm.thread_id = t.id
		  AND ` + unreadPredicateCM + `
	) AS unread_count`

	var rows []*users.StaffInboxThread
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(`users.staff_message_threads AS "t"`).
		ColumnExpr(`t.id AS thread_id`).
		ColumnExpr(`t.tenant_id AS tenant_id`).
		ColumnExpr(`t.last_message_at AS last_message_at`).
		ColumnExpr(`t.last_message_body AS last_message_body`).
		ColumnExpr(`t.last_sender_account_id AS last_sender_account_id`).
		ColumnExpr(`other.account_id AS counterpart_account_id`).
		ColumnExpr(`COALESCE(NULLIF(btrim(COALESCE(person.first_name, '') || ' ' || COALESCE(person.last_name, '')), ''), 'Unbekannt') AS counterpart_name`).
		ColumnExpr(unreadSub, accountID).
		// The viewer must be a participant — this join IS the authorization
		// filter for the whole inbox, alongside the tenant predicate.
		Join(`JOIN users.staff_message_participants AS "mine"
			ON mine.thread_id = t.id AND mine.account_id = ?`, accountID).
		Join(`JOIN users.staff_message_participants AS "other"
				ON other.thread_id = t.id AND other.account_id <> ?`, accountID).
		Join(`LEFT JOIN users.persons AS "person"
				ON person.account_id = other.account_id
				AND person.tenant_id = t.tenant_id
				AND person.deleted_at IS NULL`).
		Join(`LEFT JOIN users.staff_message_reads AS "r"
			ON r.thread_id = t.id AND r.account_id = ?`, accountID).
		Where(`t.last_message_at IS NOT NULL`).
		OrderExpr(`t.last_message_at DESC`)

	query = base.WithTenantFilter(ctx, query, "t")

	if onlyUnread {
		query = query.Where(`EXISTS (
			SELECT 1 FROM users.staff_messages um
			WHERE um.thread_id = t.id AND `+unreadPredicateUM+`
		)`, accountID)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list staff message inbox", Err: base.TranslateNotFound(err)}
	}
	return rows, nil
}

// ListMessageableStaff returns the accounts the viewer may write to: people
// with an ACTIVE mapping to the current tenant, excluding the viewer.
//
// "Active" is the whole access rule for V1 — a colleague whose mapping went
// inactive (left the school) disappears from the picker and can no longer be
// addressed, while the existing conversation history stays readable.
func (r *StaffMessageReadRepository) ListMessageableStaff(ctx context.Context, viewerAccountID int64) ([]*users.MessageableStaff, error) {
	var rows []*users.MessageableStaff
	query, err := r.colleagueQuery(ctx, base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(`users.persons AS "person"`).
		ColumnExpr(`at.account_id AS account_id`).
		ColumnExpr(`COALESCE(NULLIF(btrim(COALESCE(person.first_name, '') || ' ' || COALESCE(person.last_name, '')), ''), 'Unbekannt') AS name`).
		Where(`at.account_id <> ?`, viewerAccountID).
		OrderExpr(`name ASC`))
	if err != nil {
		return nil, err
	}

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list messageable staff", Err: base.TranslateNotFound(err)}
	}
	return rows, nil
}

// IsMessageableStaff reports whether the account may take part in an internal
// conversation at the current school. Authorization predicate for opening one.
//
// It MUST stay the exact predicate ListMessageableStaff filters the picker by,
// which is why both use colleagueQuery. Two things an active school mapping
// does NOT prove:
//   - a guardian who accepts an invitation gets exactly that row
//     (services/auth/guardian_invitation_service.go) and no persons row;
//   - a child or guest CAN carry a person row and an account, so the persons
//     join alone still admits them.
//
// Only the users.staff relation makes someone a colleague.
//
// The method is deliberately not called IsActiveTenantMember any more: that name
// described the query, not the question, and invited exactly this gap.
func (r *StaffMessageReadRepository) IsMessageableStaff(ctx context.Context, accountID int64) (bool, error) {
	query, err := r.colleagueQuery(ctx, base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`users.persons AS "person"`).
		ColumnExpr(`1`).
		Where(`at.account_id = ?`, accountID).
		Limit(1))
	if err != nil {
		return false, err
	}
	exists, existsErr := query.Exists(ctx)
	err = existsErr
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "check messageable staff", Err: base.TranslateNotFound(err)}
	}
	return exists, nil
}

// StaffRoleKinds classifies accounts by their roles at the current tenant.
//
// The Identity & Access owner decides which roles count: "admin" is the
// system admin role itself or any custom role whose base_role is admin,
// "lehrkraft" the platform system role of that name (narrowed to system roles
// exactly like services/auth.IsLehrkraftSystemRole). Precedence admin >
// lehrkraft > staff, see the StaffRoleKind constants.
func (r *StaffMessageReadRepository) StaffRoleKinds(ctx context.Context, accountIDs []int64) (map[int64]string, error) {
	out := make(map[int64]string, len(accountIDs))
	for _, id := range accountIDs {
		out[id] = users.StaffRoleKindStaff
	}
	if len(accountIDs) == 0 {
		return out, nil
	}
	if r.identity.RoleClasses == nil {
		return nil, &modelBase.DatabaseError{Op: "resolve staff role kinds", Err: errors.New("role class query is required")}
	}

	rows, err := r.identity.RoleClasses(ctx, tenant.FromContext(ctx), accountIDs)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "resolve staff role kinds", Err: err}
	}
	for _, row := range rows {
		switch {
		case row.IsAdmin:
			out[row.AccountID] = users.StaffRoleKindAdmin
		case row.IsLehrkraft:
			out[row.AccountID] = users.StaffRoleKindLehrkraft
		}
	}
	return out, nil
}
