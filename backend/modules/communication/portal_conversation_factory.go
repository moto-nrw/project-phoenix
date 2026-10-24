package communication

import "context"

type portalConversationCallbacks struct {
	appendMessage func(context.Context, *PortalParentMessage) error
	markRead      func(context.Context, int64, int64, int64, bool, []*PortalParentMessage) (bool, error)
	decorate      func(context.Context, int64, int64, []*PortalParentMessage)
	broadcast     func(int64, int64, int64, int64)
	broadcastRead func(int64, int64, int64, int64)
}

// NewPortalConversationCore binds the shared conversation through native values.
func NewPortalConversationCore(appendMessage func(context.Context, *PortalParentMessage) error, markRead func(context.Context, int64, int64, int64, bool, []*PortalParentMessage) (bool, error), decorate func(context.Context, int64, int64, []*PortalParentMessage), broadcast, broadcastRead func(int64, int64, int64, int64)) PortalConversationCore {
	return portalConversationCallbacks{appendMessage: appendMessage, markRead: markRead, decorate: decorate, broadcast: broadcast, broadcastRead: broadcastRead}
}
func (c portalConversationCallbacks) PortalAppendMessage(ctx context.Context, v *PortalParentMessage) error {
	return c.appendMessage(ctx, v)
}
func (c portalConversationCallbacks) PortalMarkReadToNewest(ctx context.Context, tenantID, threadID, accountID int64, staff bool, rows []*PortalParentMessage) (bool, error) {
	return c.markRead(ctx, tenantID, threadID, accountID, staff, rows)
}
func (c portalConversationCallbacks) PortalDecorateReadReceipts(ctx context.Context, threadID, other int64, rows []*PortalParentMessage) {
	c.decorate(ctx, threadID, other, rows)
}
func (c portalConversationCallbacks) Broadcast(a, b, d, e int64)     { c.broadcast(a, b, d, e) }
func (c portalConversationCallbacks) BroadcastRead(a, b, d, e int64) { c.broadcastRead(a, b, d, e) }
