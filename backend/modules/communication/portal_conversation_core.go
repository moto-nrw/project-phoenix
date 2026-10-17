package communication

import (
	"context"
)

type PortalConversationCore interface {
	PortalAppendMessage(ctx context.Context, msg *PortalParentMessage) error
	PortalMarkReadToNewest(ctx context.Context, tenantID int64, threadID int64, accountID int64, staffReader bool, messages []*PortalParentMessage) (bool, error)
	PortalDecorateReadReceipts(ctx context.Context, threadID int64, otherAccountID int64, messages []*PortalParentMessage)
	Broadcast(tenantID int64, guardianAccountID int64, threadID int64, studentID int64)
	BroadcastRead(tenantID int64, guardianAccountID int64, threadID int64, studentID int64)
}
