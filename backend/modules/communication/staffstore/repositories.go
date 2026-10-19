package staffstore

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/staffpostgres"
	"github.com/uptrace/bun"
)

func NewStaffMessageThreadRepository(db *bun.DB) communication.StaffThreadRecords {
	return staffpostgres.NewThreadStore(postgresDatabase(db))
}
func NewStaffMessageRepository(db *bun.DB) communication.StaffMessageRecords {
	return staffpostgres.NewMessageStore(postgresDatabase(db))
}

// NewStaffMessageReadRepository binds the existing cursors and inbox to the
// exact colleague relation supplied by People Directory at the root.
func NewStaffMessageReadRepository(db *bun.DB, colleagues communication.StaffColleagueReads, inbox StaffInbox) communication.StaffReadRecords {
	if colleagues == nil {
		panic("communication staff store: colleague directory is required")
	}
	return &readRepository{StaffColleagueReads: colleagues, cursors: staffpostgres.NewReadCursorStore(postgresDatabase(db)), inbox: inbox}
}

type readRepository struct {
	communication.StaffColleagueReads
	cursors *staffpostgres.ReadCursorStore
	inbox   StaffInbox
}

func (r *readRepository) MarkStaffReadUpTo(ctx context.Context, id, accountID int64, at time.Time, messageID int64) error {
	return r.cursors.MarkReadUpTo(ctx, id, accountID, at, messageID)
}
func (r *readRepository) StaffUnreadCount(ctx context.Context, accountID int64) (int, error) {
	return r.inbox.UnreadCount(ctx, accountID)
}
func (r *readRepository) ListStaffInbox(ctx context.Context, accountID int64, unread bool) ([]*communication.StaffInboxRecord, error) {
	values, err := r.inbox.ListInbox(ctx, accountID, unread)
	if err != nil {
		return nil, err
	}
	rows := make([]*communication.StaffInboxRecord, 0, len(values))
	for _, value := range values {
		rows = append(rows, &communication.StaffInboxRecord{ThreadID: value.ThreadID, TenantID: value.TenantID, CounterpartAccountID: value.CounterpartAccountID, CounterpartName: value.CounterpartName, LastMessageAt: value.LastMessageAt, LastMessageBody: value.LastMessageBody, LastSenderAccountID: value.LastSenderAccountID, UnreadCount: value.UnreadCount})
	}
	return rows, nil
}
