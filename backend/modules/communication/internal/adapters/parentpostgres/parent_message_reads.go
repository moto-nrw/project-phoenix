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

// clearUnreadForStaffSQL moves each account's personal clear boundary to the
// exact counterpart message chosen by the inbox snapshot. Re-reading
// parent_messages here could include a guardian message that committed after
// that snapshot. The read cursor is left alone, so "Von der OGS gelesen" does
// not change (#3673); a row created here starts its read cursor before any
// message. The conflict guard keeps the boundary from moving backward and from
// reporting an unchanged row.
// Bind args: tenantID, accountID, threadIDs, clearedAt values, messageIDs.
const clearUnreadForStaffSQL = `
INSERT INTO users.parent_message_reads AS pmr (tenant_id, thread_id, account_id, last_read_at, last_read_message_id, cleared_up_to_at, cleared_up_to_message_id)
SELECT ?, bound.thread_id, ?, '1970-01-01'::timestamptz, 0, bound.cleared_at, bound.message_id
FROM unnest(?::bigint[], ?::timestamptz[], ?::bigint[]) AS bound(thread_id, cleared_at, message_id)
ON CONFLICT (thread_id, account_id) DO UPDATE
SET cleared_up_to_at = EXCLUDED.cleared_up_to_at,
    cleared_up_to_message_id = EXCLUDED.cleared_up_to_message_id
WHERE (EXCLUDED.cleared_up_to_at, EXCLUDED.cleared_up_to_message_id) >
      (COALESCE(pmr.cleared_up_to_at, '1970-01-01'::timestamptz), COALESCE(pmr.cleared_up_to_message_id, 0))
RETURNING pmr.thread_id`

// ClearUnreadForStaff moves each personal clear boundary to its selected inbox
// bound, never to NOW() and never backward. It returns threads whose boundary
// moved.
func (s *ReadCursorStore) ClearUnreadForStaff(ctx context.Context, tenantID, accountID int64, bounds []domain.ReadCursorBound) ([]int64, error) {
	if len(bounds) == 0 {
		return nil, nil
	}
	threadIDs := make([]int64, 0, len(bounds))
	clearedAt := make([]time.Time, 0, len(bounds))
	messageIDs := make([]int64, 0, len(bounds))
	for _, bound := range bounds {
		threadIDs = append(threadIDs, bound.ThreadID)
		clearedAt = append(clearedAt, bound.ReadAt)
		messageIDs = append(messageIDs, bound.MessageID)
	}
	db, _, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var cleared []int64
	if err := db.NewRaw(clearUnreadForStaffSQL,
		tenantID, accountID, pgdialect.Array(threadIDs), pgdialect.Array(clearedAt), pgdialect.Array(messageIDs),
	).Scan(ctx, &cleared); err != nil {
		return nil, fmt.Errorf("clear parent message unread for staff: %w", err)
	}
	return cleared, nil
}
