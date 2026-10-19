package parentpostgres

import (
	"context"
	"time"

	recordports "github.com/moto-nrw/project-phoenix/modules/communication/ports"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"
	"github.com/uptrace/bun"
)

// ParentMessageReads answers the inbox, unread, header and receipt reads plus
// the two cursor writes.
//
// The three staff-gated projections need the school's staff accounts, which
// School Membership owns, so they are exposed as *WithStaff methods and the
// composition root completes the repository contract over that owner.
type ParentMessageReads struct {
	cursors *ReadCursorStore
	threads *ThreadStore
	recordports.ParentInboxReads
}

// NewParentMessageReadRepository builds the read-side store over the
// guardian-link row source.
func NewParentMessageReadRepository(db *bun.DB, inbox recordports.ParentInboxReads) *ParentMessageReads {
	return &ParentMessageReads{
		cursors:          NewReadCursorStore(NewParentRecordDatabase(db)),
		threads:          NewThreadStore(NewParentRecordDatabase(db)),
		ParentInboxReads: inbox,
	}
}

func (r *ParentMessageReads) MarkReadUpTo(ctx context.Context, tenantID, threadID, accountID int64, readAt time.Time, readMessageID int64) (bool, error) {
	return r.cursors.MarkReadUpTo(ctx, tenantID, threadID, accountID, readAt, readMessageID)
}

func (r *ParentMessageReads) ClearUnreadForStaff(ctx context.Context, tenantID, accountID int64, bounds []communication.PortalReadCursorBound) ([]int64, error) {
	selected := make([]domain.ReadCursorBound, 0, len(bounds))
	for _, bound := range bounds {
		selected = append(selected, domain.ReadCursorBound{
			ThreadID: bound.ThreadID, ReadAt: bound.ReadAt, MessageID: bound.MessageID,
		})
	}
	return r.cursors.ClearUnreadForStaff(ctx, tenantID, accountID, selected)
}

func (r *ParentMessageReads) MarkStaffHandledUpTo(ctx context.Context, tenantID, threadID int64, handledAt time.Time, handledMessageID int64) error {
	return r.threads.MarkStaffHandledUpTo(ctx, tenantID, threadID, handledAt, handledMessageID)
}

func (r *ParentMessageReads) MarkStaffUnread(ctx context.Context, tenantID, threadID, accountID int64) error {
	return r.threads.MarkStaffUnread(ctx, tenantID, threadID, accountID)
}

func (r *ParentMessageReads) ClearStaffUnreadMark(ctx context.Context, tenantID, threadID int64, observedAt time.Time) (bool, error) {
	return r.threads.ClearStaffUnreadMark(ctx, tenantID, threadID, observedAt)
}
