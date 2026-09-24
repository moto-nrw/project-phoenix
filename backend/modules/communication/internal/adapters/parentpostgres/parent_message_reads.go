package parentpostgres

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

const parentReadTable = "users.parent_message_reads"

// ReadCursorStore owns each reader's position in a thread. The cursor reads
// that render the inbox and the receipts live in the inbox projection; this
// store holds the single write path.
type ReadCursorStore struct{ store }

// NewReadCursorStore builds the read-cursor store over the ambient transaction
// runtime.
func NewReadCursorStore(database Database) *ReadCursorStore {
	return &ReadCursorStore{store: newStore(database)}
}

type parentReadRow struct {
	bun.BaseModel     `bun:"table:users.parent_message_reads,alias:pmr"`
	TenantID          int64     `bun:"tenant_id,notnull"`
	ThreadID          int64     `bun:"thread_id,pk"`
	AccountID         int64     `bun:"account_id,pk"`
	LastReadAt        time.Time `bun:"last_read_at,notnull"`
	LastReadMessageID int64     `bun:"last_read_message_id,notnull"`
}

// MarkReadUpTo advances the reader's cursor to the composite (readAt,
// readMessageID) — the created_at AND id of the newest message actually shown —
// instead of NOW(), and never backward. A NOW() cursor would fall past a
// message that committed between the caller's snapshot and this write, silently
// dropping it from the reader's unread count although it was never shown.
//
// It returns whether the cursor actually ADVANCED. The strictly-greater test
// lives in the ON CONFLICT DO UPDATE ... WHERE, so a non-advance touches NO row
// and RowsAffected is exactly "did the cursor move"; a CASE in SET would always
// write and lose that signal. The read-receipt push gates on this, which is what
// stops the receipt event from ping-ponging with the refetch it triggers.
func (s *ReadCursorStore) MarkReadUpTo(ctx context.Context, tenantID, threadID, accountID int64, readAt time.Time, readMessageID int64) (bool, error) {
	db, _, err := s.database(ctx)
	if err != nil {
		return false, err
	}
	row := &parentReadRow{
		TenantID:          tenantID,
		ThreadID:          threadID,
		AccountID:         accountID,
		LastReadAt:        readAt,
		LastReadMessageID: readMessageID,
	}
	// Compare the incoming composite against the stored one as a tuple, not as
	// two independent GREATEST(): a newer timestamp must not mix with an older
	// id and corrupt the cursor.
	const advance = `(EXCLUDED.last_read_at, EXCLUDED.last_read_message_id) > ` +
		`(parent_message_reads.last_read_at, parent_message_reads.last_read_message_id)`
	result, err := db.NewInsert().
		Model(row).
		ModelTableExpr(parentReadTable).
		On("CONFLICT (thread_id, account_id) DO UPDATE").
		Set("last_read_at = EXCLUDED.last_read_at").
		Set("last_read_message_id = EXCLUDED.last_read_message_id").
		Where(advance).
		Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("mark parent message thread read up to: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("mark parent message thread read up to: %w", err)
	}
	return affected > 0, nil
}

// markThreadsReadForStaffSQL advances one staff reader's cursor in many threads
// at once. Per thread it picks the newest message in the reader's unread set,
// the same bound MarkReadToNewest applies to a single open: a guardian-side
// message (a system event counts by the side that triggered it), never a
// request_created notice, never the reader's own plain message. The counterpart
// test mirrors usersModels.IsCounterpartMessage for a staff reader and the
// unread predicates of the inbox projection.
//
// The statement reads the messages committed when it starts, so a guardian
// message that arrives later stays unread. The ON CONFLICT guard compares the
// composite, so the cursor never moves backward and an unchanged row is not
// returned.
//
// Bind args: accountID, tenantID, threadIDs, accountID.
const markThreadsReadForStaffSQL = `
INSERT INTO users.parent_message_reads AS pmr (tenant_id, thread_id, account_id, last_read_at, last_read_message_id)
SELECT DISTINCT ON (m.thread_id) m.tenant_id, m.thread_id, ?, m.created_at, m.id
FROM users.parent_messages m
WHERE m.tenant_id = ?
  AND m.thread_id IN (?)
  AND (m.sender_kind = 'guardian' OR (m.sender_kind = 'system' AND m.event_actor_kind = 'guardian'))
  AND m.event_type IS DISTINCT FROM 'request_created'
  AND (m.sender_kind = 'system' OR m.sender_account_id <> ?)
ORDER BY m.thread_id, m.created_at DESC, m.id DESC
ON CONFLICT (thread_id, account_id) DO UPDATE
SET last_read_at = EXCLUDED.last_read_at,
    last_read_message_id = EXCLUDED.last_read_message_id
WHERE (EXCLUDED.last_read_at, EXCLUDED.last_read_message_id) > (pmr.last_read_at, pmr.last_read_message_id)
RETURNING pmr.thread_id`

// MarkThreadsReadForStaff advances the staff reader's cursor in each thread to
// the newest guardian-side message they did not author, never to NOW() and
// never backward. It returns the threads whose cursor moved.
func (s *ReadCursorStore) MarkThreadsReadForStaff(ctx context.Context, tenantID, accountID int64, threadIDs []int64) ([]int64, error) {
	if len(threadIDs) == 0 {
		return nil, nil
	}
	db, _, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var advanced []int64
	if err := db.NewRaw(markThreadsReadForStaffSQL,
		accountID, tenantID, bun.List(threadIDs), accountID,
	).Scan(ctx, &advanced); err != nil {
		return nil, fmt.Errorf("mark parent message threads read for staff: %w", err)
	}
	return advanced, nil
}
