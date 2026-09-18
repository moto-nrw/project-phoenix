package staffpostgres

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

const staffReadsTable = "users.staff_message_reads"

// ReadCursorStore owns each reader's position in a staff conversation. The
// unread counts derived from it live in the staffinbox projection; this store
// holds the single write path.
type ReadCursorStore struct{ store }

// NewReadCursorStore builds the read-cursor store over the ambient transaction
// runtime.
func NewReadCursorStore(database Database) *ReadCursorStore {
	return &ReadCursorStore{store: newStore(database)}
}

// staffReadRow is a composite-key row: the cursor is identified by
// (thread_id, account_id), not by an autoincrement id.
type staffReadRow struct {
	bun.BaseModel     `bun:"table:users.staff_message_reads,alias:smr"`
	TenantID          int64     `bun:"tenant_id,notnull"`
	ThreadID          int64     `bun:"thread_id,pk"`
	AccountID         int64     `bun:"account_id,pk"`
	LastReadAt        time.Time `bun:"last_read_at,notnull"`
	LastReadMessageID int64     `bun:"last_read_message_id,notnull"`
}

// MarkReadUpTo advances the account's cursor in one thread to the supplied
// composite.
//
// The strictly-greater test lives in the ON CONFLICT DO UPDATE ... WHERE so a
// non-advance touches NO row. A CASE in SET would always write and lose that
// property, letting an out-of-order request drag the cursor backwards and
// resurrect already-read messages in the badge.
func (s *ReadCursorStore) MarkReadUpTo(ctx context.Context, threadID, accountID int64, at time.Time, messageID int64) error {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	row := &staffReadRow{
		TenantID:          tenantID,
		ThreadID:          threadID,
		AccountID:         accountID,
		LastReadAt:        at,
		LastReadMessageID: messageID,
	}

	const advance = `(EXCLUDED.last_read_at, EXCLUDED.last_read_message_id) > ` +
		`(staff_message_reads.last_read_at, staff_message_reads.last_read_message_id)`

	if _, err := db.NewInsert().
		Model(row).
		ModelTableExpr(staffReadsTable).
		On("CONFLICT (thread_id, account_id) DO UPDATE").
		Set("last_read_at = EXCLUDED.last_read_at").
		Set("last_read_message_id = EXCLUDED.last_read_message_id").
		Where(advance).
		Exec(ctx); err != nil {
		return fmt.Errorf("mark staff message thread read up to: %w", err)
	}
	return nil
}
