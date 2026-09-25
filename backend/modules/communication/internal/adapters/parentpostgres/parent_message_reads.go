package parentpostgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
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

// markThreadsReadForStaffSQL writes only the exact counterpart messages chosen
// by the inbox snapshot. Re-reading parent_messages here could include a
// guardian message that committed after that snapshot. The conflict guard
// prevents a cursor from moving backward or reporting an unchanged row.
// Bind args: tenantID, accountID, threadIDs, readAt values, messageIDs.
const markThreadsReadForStaffSQL = `
INSERT INTO users.parent_message_reads AS pmr (tenant_id, thread_id, account_id, last_read_at, last_read_message_id)
SELECT ?, bound.thread_id, ?, bound.read_at, bound.message_id
FROM unnest(?::bigint[], ?::timestamptz[], ?::bigint[]) AS bound(thread_id, read_at, message_id)
ON CONFLICT (thread_id, account_id) DO UPDATE
SET last_read_at = EXCLUDED.last_read_at,
    last_read_message_id = EXCLUDED.last_read_message_id
WHERE (EXCLUDED.last_read_at, EXCLUDED.last_read_message_id) > (pmr.last_read_at, pmr.last_read_message_id)
RETURNING pmr.thread_id`

// MarkThreadsReadForStaff advances each cursor to its selected inbox bound,
// never to NOW() and never backward. It returns threads whose cursor moved.
func (s *ReadCursorStore) MarkThreadsReadForStaff(ctx context.Context, tenantID, accountID int64, bounds []domain.ReadCursorBound) ([]int64, error) {
	if len(bounds) == 0 {
		return nil, nil
	}
	threadIDs := make([]int64, 0, len(bounds))
	readAt := make([]time.Time, 0, len(bounds))
	messageIDs := make([]int64, 0, len(bounds))
	for _, bound := range bounds {
		threadIDs = append(threadIDs, bound.ThreadID)
		readAt = append(readAt, bound.ReadAt)
		messageIDs = append(messageIDs, bound.MessageID)
	}
	db, _, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var advanced []int64
	if err := db.NewRaw(markThreadsReadForStaffSQL,
		tenantID, accountID, pgdialect.Array(threadIDs), pgdialect.Array(readAt), pgdialect.Array(messageIDs),
	).Scan(ctx, &advanced); err != nil {
		return nil, fmt.Errorf("mark parent message threads read for staff: %w", err)
	}
	return advanced, nil
}
