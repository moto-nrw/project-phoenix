// Package staffinbox implements the tenant-safe staff-conversation projection:
// the reader's inbox and the sidebar unread badge (#3221).
//
// It is a read-only projection because the inbox joins Communication's
// staff_message* tables with People Directory's person rows to name the
// counterpart. It never writes; the thread, message and cursor writes stay with
// their owner in staffpostgres. Every statement carries the tenant predicate on
// top of RLS and names the users schema explicitly for the least-privilege
// phoenix_tenant role.
package staffinbox

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/staffmessageinbox/contract"

	"github.com/uptrace/bun"
)

// Database resolves the caller connection and school; school-specific queries
// reject an absent school instead of removing the tenant predicate.
type Database func(context.Context) (bun.IDB, int64, error)

// Projection answers the staff-conversation reads that span owners.
type Projection struct{ database Database }

// New builds the projection over the ambient transaction runtime.
func New(database Database) *Projection {
	if database == nil {
		panic("communication staff inbox: database runtime is required")
	}
	return &Projection{database: database}
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
// reader's account id. The last term hides a group's messages from before the
// reader joined (#4776): they are neither shown nor unread. `mine` is the
// reader's own participant row in every query.
const (
	unreadPredicateM = `(m.created_at, m.id) > (COALESCE(r.last_read_at, '1970-01-01'::timestamptz), COALESCE(r.last_read_message_id, 0))
		 AND m.sender_account_id <> ?
		 AND (mine.history_from IS NULL OR m.created_at >= mine.history_from)`
	unreadPredicateCM = `(cm.created_at, cm.id) > (COALESCE(r.last_read_at, '1970-01-01'::timestamptz), COALESCE(r.last_read_message_id, 0))
		 AND cm.sender_account_id <> ?
		 AND (mine.history_from IS NULL OR cm.created_at >= mine.history_from)`
	unreadPredicateUM = `(um.created_at, um.id) > (COALESCE(r.last_read_at, '1970-01-01'::timestamptz), COALESCE(r.last_read_message_id, 0))
		 AND um.sender_account_id <> ?
		 AND (mine.history_from IS NULL OR um.created_at >= mine.history_from)`
)

// withTenant requires a school and adds its predicate on top of RLS.
func withTenant[Q interface {
	Where(string, ...any) Q
	Err(error) Q
}](query Q, alias string, tenantID int64) Q {
	if tenantID <= 0 {
		return query.Err(errors.New("tenant is required"))
	}
	return query.Where(`"`+alias+`".tenant_id = ?`, tenantID)
}

// UnreadCount is the account's total unread messages across every conversation
// it takes part in — the sidebar badge.
func (p *Projection) UnreadCount(ctx context.Context, accountID int64) (int, error) {
	db, tenantID, err := p.database(ctx)
	if err != nil {
		return 0, err
	}
	query := db.NewSelect().
		TableExpr(`users.staff_messages AS "m"`).
		ColumnExpr(`COUNT(*)`).
		Join(`JOIN users.staff_message_participants AS "mine"
			ON mine.thread_id = m.thread_id AND mine.account_id = ?`, accountID).
		Join(`LEFT JOIN users.staff_message_reads AS "r"
			ON r.thread_id = m.thread_id AND r.account_id = ?`, accountID).
		Where(unreadPredicateM, accountID)
	query = withTenant(query, "m", tenantID)

	count := 0
	if err := query.Scan(ctx, &count); err != nil {
		return 0, fmt.Errorf("count unread staff messages: %w", err)
	}
	return count, nil
}

type inboxRow struct {
	ThreadID             int64      `bun:"thread_id"`
	TenantID             int64      `bun:"tenant_id"`
	CounterpartAccountID int64      `bun:"counterpart_account_id"`
	CounterpartName      string     `bun:"counterpart_name"`
	LastMessageAt        *time.Time `bun:"last_message_at"`
	LastMessageBody      string     `bun:"last_message_body"`
	LastSenderAccountID  *int64     `bun:"last_sender_account_id"`
	UnreadCount          int        `bun:"unread_count"`
	Kind                 string     `bun:"kind"`
	GroupName            string     `bun:"group_name"`
	MemberCount          int        `bun:"member_count"`
	LastSenderName       string     `bun:"last_sender_name"`
}

// ListInbox projects the account's conversations, newest activity first, with
// the counterpart and the per-thread unread count resolved.
//
// The counterpart is resolved per viewer: the same thread row renders as "Anna"
// for Ben and as "Ben" for Anna, so the join picks the participant that is NOT
// the viewer. Direct threads without any message are skipped — a get-or-create
// that was never followed by a send must not clutter the inbox.
//
// A group (#4776) is one row with its name and size; it shows from the moment
// the viewer joined, even before anyone wrote. Its preview hides a last
// message from before the viewer joined.
func (p *Projection) ListInbox(ctx context.Context, accountID int64, onlyUnread bool) ([]*contract.StaffInboxThread, error) {
	db, tenantID, err := p.database(ctx)
	if err != nil {
		return nil, err
	}

	var rows []inboxRow
	query := inboxColumns(db.NewSelect().
		Model(&rows).
		ModelTableExpr(`users.staff_message_threads AS "t"`)).
		ColumnExpr(unreadSub, accountID).
		// The viewer must be a participant — this join IS the authorization
		// filter for the whole inbox, alongside the tenant predicate.
		Join(`JOIN users.staff_message_participants AS "mine"
			ON mine.thread_id = t.id AND mine.account_id = ?`, accountID).
		Join(`LEFT JOIN users.staff_message_participants AS "other"
				ON other.thread_id = t.id AND other.account_id <> ? AND t.kind = 'direct'`, accountID).
		Join(`LEFT JOIN users.persons AS "person"
				ON person.account_id = other.account_id
				AND person.tenant_id = t.tenant_id
				AND person.deleted_at IS NULL`).
		Join(`LEFT JOIN users.staff_messages AS "last_message"
				ON last_message.id = t.last_message_id AND t.kind = 'group'`).
		Join(`LEFT JOIN users.staff_message_reads AS "r"
			ON r.thread_id = t.id AND r.account_id = ?`, accountID).
		Where(`(t.last_message_at IS NOT NULL OR t.kind = 'group')`).
		// A group nobody has written in yet sorts by when the viewer joined,
		// so a new group lands at the top.
		OrderExpr(`COALESCE(CASE WHEN ` + visibleLast + ` THEN t.last_message_at END, mine.created_at) DESC`)
	query = withTenant(query, "t", tenantID)

	if onlyUnread {
		query = query.Where(`EXISTS (
			SELECT 1 FROM users.staff_messages um
			WHERE um.thread_id = t.id AND `+unreadPredicateUM+`
		)`, accountID)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("list staff message inbox: %w", err)
	}
	threads := inboxThreads(rows)
	return threads, nil
}

const unreadSub = `(
		SELECT COUNT(*)
		FROM users.staff_messages cm
		WHERE cm.thread_id = t.id
		  AND ` + unreadPredicateCM + `
	) AS unread_count`

// visibleLast: the thread's last message as far as this viewer may see it.
const visibleLast = `(mine.history_from IS NULL OR t.last_message_at >= mine.history_from)`

// inboxColumns selects one inbox row per thread: a direct chat names its
// counterpart, a group its name, size and the sender of the last message the
// viewer may see.
func inboxColumns(query *bun.SelectQuery) *bun.SelectQuery {
	return query.
		ColumnExpr(`t.id AS thread_id`).
		ColumnExpr(`t.tenant_id AS tenant_id`).
		ColumnExpr(`t.kind AS kind`).
		ColumnExpr(`COALESCE(t.name, '') AS group_name`).
		ColumnExpr(`CASE WHEN ` + visibleLast + ` THEN t.last_message_at END AS last_message_at`).
		ColumnExpr(`CASE WHEN ` + visibleLast + ` THEN t.last_message_body ELSE '' END AS last_message_body`).
		ColumnExpr(`CASE WHEN ` + visibleLast + ` THEN t.last_sender_account_id END AS last_sender_account_id`).
		ColumnExpr(`CASE WHEN ` + visibleLast + ` THEN COALESCE(last_message.sender_name, '') ELSE '' END AS last_sender_name`).
		ColumnExpr(`COALESCE(other.account_id, 0) AS counterpart_account_id`).
		ColumnExpr(`CASE WHEN t.kind = 'group' THEN '' ELSE COALESCE(NULLIF(btrim(COALESCE(person.first_name, '') || ' ' || COALESCE(person.last_name, '')), ''), 'Unbekannt') END AS counterpart_name`).
		ColumnExpr(`(SELECT COUNT(*) FROM users.staff_message_participants members WHERE members.thread_id = t.id) AS member_count`)
}

func inboxThreads(rows []inboxRow) []*contract.StaffInboxThread {
	threads := make([]*contract.StaffInboxThread, 0, len(rows))
	for _, row := range rows {
		threads = append(threads, &contract.StaffInboxThread{
			ThreadID: row.ThreadID, TenantID: row.TenantID,
			CounterpartAccountID: row.CounterpartAccountID, CounterpartName: row.CounterpartName,
			LastMessageAt: row.LastMessageAt, LastMessageBody: row.LastMessageBody,
			LastSenderAccountID: row.LastSenderAccountID, UnreadCount: row.UnreadCount,
			Kind: row.Kind, GroupName: row.GroupName, MemberCount: row.MemberCount, LastSenderName: row.LastSenderName,
		})
	}
	return threads
}
