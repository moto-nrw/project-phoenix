package staffpostgres

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

type readRepository struct {
	communication.StaffColleagueReads
	cursors *ReadCursorStore
	inbox   communication.StaffInboxReads
}

func (r *readRepository) MarkStaffReadUpTo(ctx context.Context, id, accountID int64, at time.Time, messageID int64) error {
	return r.cursors.MarkReadUpTo(ctx, id, accountID, at, messageID)
}
func (r *readRepository) StaffUnreadCount(ctx context.Context, accountID int64) (int, error) {
	return r.inbox.UnreadCount(ctx, accountID)
}
func (r *readRepository) StaffUnreadDirectCount(ctx context.Context, accountID int64) (int, error) {
	return r.inbox.UnreadDirectCount(ctx, accountID)
}
func (r *readRepository) ListStaffInbox(ctx context.Context, accountID int64, unread bool) ([]*communication.StaffInboxRecord, error) {
	return r.inbox.ListInbox(ctx, accountID, unread)
}

func NewReadRecords(colleagues communication.StaffColleagueReads, cursors *ReadCursorStore, inbox communication.StaffInboxReads) communication.StaffReadRecords {
	return &readRepository{StaffColleagueReads: colleagues, cursors: cursors, inbox: inbox}
}
