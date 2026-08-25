package users

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// StaffMessageReadRepository owns the read cursors and every projection derived
// from them (inbox rows, unread badge, recipient picker).
type StaffMessageReadRepository struct {
	db *bun.DB
}

// NewStaffMessageReadRepository wires a fresh repository.
//
// Composite-key table, so this does NOT embed base.Repository[T]: the generic
// helpers assume a single autoincrement id. It stays a plain struct with
// explicit queries, mirroring ParentMessageReadRepository.
func NewStaffMessageReadRepository(db *bun.DB) users.StaffMessageReadRepository {
	return &StaffMessageReadRepository{db: db}
}

// unreadPredicate is the correctness core of every unread number in this
// feature: "message <alias> is strictly after the reader's cursor AND the
// reader did not write it".
//
// It lives in exactly ONE place on purpose. The inbox's per-thread unread_count
// column, the onlyUnread filter, and the sidebar badge all consume it, so a fix
// here cannot land in one copy and silently skip the others — which is how an
// inbox count and a sidebar badge start disagreeing.
//
// The comparison is a TUPLE, not two independent tests: clock_timestamp() can
// stamp two messages with the same created_at, and the message list breaks
// those ties by id. Comparing the pair keeps a newer timestamp from mixing with
// an older id and skipping a message out of the unread set. The `?` binds the
// reader's account id.
func unreadPredicate(alias string) string {
	return fmt.Sprintf(
		`(%[1]s.created_at, %[1]s.id) > (COALESCE(r.last_read_at, '1970-01-01'::timestamptz), COALESCE(r.last_read_message_id, 0))
		 AND %[1]s.sender_account_id <> ?`,
		alias,
	)
}

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
		return &modelBase.DatabaseError{Op: "mark staff message thread read up to", Err: err}
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
		Where(unreadPredicate("m"), accountID)

	query = base.WithTenantFilter(ctx, query, "m")

	count := 0
	if err := query.Scan(ctx, &count); err != nil {
		return 0, &modelBase.DatabaseError{Op: "count unread staff messages", Err: err}
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
	unreadSub := fmt.Sprintf(`(
		SELECT COUNT(*)
		FROM users.staff_messages cm
		WHERE cm.thread_id = t.id
		  AND %s
	) AS unread_count`, unreadPredicate("cm"))

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
			ON person.account_id = other.account_id AND person.deleted_at IS NULL`).
		Join(`LEFT JOIN users.staff_message_reads AS "r"
			ON r.thread_id = t.id AND r.account_id = ?`, accountID).
		Where(`t.last_message_at IS NOT NULL`).
		OrderExpr(`t.last_message_at DESC`)

	query = base.WithTenantFilter(ctx, query, "t")

	if onlyUnread {
		query = query.Where(fmt.Sprintf(`EXISTS (
			SELECT 1 FROM users.staff_messages um
			WHERE um.thread_id = t.id AND %s
		)`, unreadPredicate("um")), accountID)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list staff message inbox", Err: err}
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
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(`auth.account_tenants AS "at"`).
		ColumnExpr(`at.account_id AS account_id`).
		ColumnExpr(`COALESCE(NULLIF(btrim(COALESCE(person.first_name, '') || ' ' || COALESCE(person.last_name, '')), ''), 'Unbekannt') AS name`).
		Join(`JOIN users.persons AS "person"
			ON person.account_id = at.account_id AND person.deleted_at IS NULL`).
		Where(`at.status = ?`, authModels.AccountTenantStatusActive).
		Where(`at.account_id <> ?`, viewerAccountID).
		Where(`at.tenant_id = ?`, tenant.FromContext(ctx)).
		OrderExpr(`name ASC`)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list messageable staff", Err: err}
	}
	return rows, nil
}

// IsActiveTenantMember reports whether the account has an active mapping to the
// current tenant. Authorization predicate for opening a conversation.
func (r *StaffMessageReadRepository) IsActiveTenantMember(ctx context.Context, accountID int64) (bool, error) {
	exists, err := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`auth.account_tenants AS "at"`).
		ColumnExpr(`1`).
		Where(`at.account_id = ?`, accountID).
		Where(`at.tenant_id = ?`, tenant.FromContext(ctx)).
		Where(`at.status = ?`, authModels.AccountTenantStatusActive).
		Limit(1).
		Exists(ctx)
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "check active tenant membership", Err: err}
	}
	return exists, nil
}
