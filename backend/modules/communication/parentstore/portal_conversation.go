package parentstore

import (
	"context"

	models "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/communication"
)

type retainedPortalConversation interface {
	AppendMessage(context.Context, *models.ParentMessage) error
	MarkReadToNewest(context.Context, int64, int64, int64, bool, []*models.ParentMessage) (bool, error)
	DecorateReadReceipts(context.Context, int64, int64, []*models.ParentMessage)
	Broadcast(int64, int64, int64, int64)
	BroadcastRead(int64, int64, int64, int64)
}

// NewPortalConversationCore exposes the same bound append and receipt policy
// through native public values. It does not construct another conversation.
func NewPortalConversationCore(source retainedPortalConversation) communication.PortalConversationCore {
	if source == nil {
		return nil
	}
	return portalConversationCore{source}
}

type portalConversationCore struct{ retainedPortalConversation }

func (c portalConversationCore) PortalAppendMessage(ctx context.Context, value *communication.PortalParentMessage) error {
	row, err := portalToRetainedParentMessagePointer(value)
	if err != nil {
		return err
	}
	err = c.AppendMessage(ctx, row)
	mapped, mapErr := portalFromRetainedParentMessagePointer(row)
	if value != nil && mapped != nil {
		*value = *mapped
	}
	if err != nil {
		return err
	}
	return mapErr
}
func (c portalConversationCore) PortalMarkReadToNewest(ctx context.Context, tenantID, threadID, accountID int64, staffReader bool, messages []*communication.PortalParentMessage) (bool, error) {
	return c.MarkReadToNewest(ctx, tenantID, threadID, accountID, staffReader, portalCursorMessages(messages))
}
func (c portalConversationCore) PortalDecorateReadReceipts(ctx context.Context, threadID, otherAccountID int64, messages []*communication.PortalParentMessage) {
	rows := portalCursorMessages(messages)
	c.DecorateReadReceipts(ctx, threadID, otherAccountID, rows)
	for i, row := range rows {
		if row != nil && messages[i] != nil {
			messages[i].ReadByStaff = row.ReadByStaff
			messages[i].ReadByGuardian = row.ReadByGuardian
		}
	}
}

// The shared read policy consumes cursor/actor facts and changes only receipts.
// Payload is not a cursor fact; keep the native JSON object untouched.
func portalCursorMessages(values []*communication.PortalParentMessage) []*models.ParentMessage {
	if values == nil {
		return nil
	}
	out := make([]*models.ParentMessage, len(values))
	for i, row := range values {
		if row != nil {
			value := portalRetainedMessage(*row, nil)
			out[i] = &value
		}
	}
	return out
}
