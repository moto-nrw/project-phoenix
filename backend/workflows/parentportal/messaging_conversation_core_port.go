package parentportal

import (
	"context"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/messaging"
)

type ConversationCore interface{ messaging.ConversationCore }
type ConversationCoreCallbacks struct {
	AppendMessage        func(ctx context.Context, msg *ParentMessage) error
	MarkReadToNewest     func(ctx context.Context, tenantID int64, threadID int64, accountID int64, staffReader bool, messages []*ParentMessage) (bool, error)
	DecorateReadReceipts func(ctx context.Context, threadID int64, otherAccountID int64, messages []*ParentMessage)
	Broadcast            func(tenantID int64, guardianAccountID int64, threadID int64, studentID int64, audience string)
	BroadcastRead        func(tenantID int64, guardianAccountID int64, threadID int64, studentID int64, audience string)
	// LeadershipOption reads the school's leadership option (#5200). Without
	// it the option is closed: no choice is offered and no new leadership
	// conversation begins.
	LeadershipOption func(ctx context.Context) (LeadershipOption, error)
}

func NewConversationCore(c ConversationCoreCallbacks) ConversationCore {
	return conversationCoreBinding{c}
}

type conversationCoreBinding struct{ callbacks ConversationCoreCallbacks }

func (p conversationCoreBinding) AppendMessage(ctx context.Context, msg *messaging.ParentMessage) error {
	return p.callbacks.AppendMessage(ctx, msg)
}
func (p conversationCoreBinding) MarkReadToNewest(ctx context.Context, tenantID int64, threadID int64, accountID int64, staffReader bool, messages []*messaging.ParentMessage) (bool, error) {
	return p.callbacks.MarkReadToNewest(ctx, tenantID, threadID, accountID, staffReader, messages)
}
func (p conversationCoreBinding) DecorateReadReceipts(ctx context.Context, threadID int64, otherAccountID int64, messages []*messaging.ParentMessage) {
	p.callbacks.DecorateReadReceipts(ctx, threadID, otherAccountID, messages)
}
func (p conversationCoreBinding) Broadcast(tenantID int64, guardianAccountID int64, threadID int64, studentID int64, audience string) {
	p.callbacks.Broadcast(tenantID, guardianAccountID, threadID, studentID, audience)
}
func (p conversationCoreBinding) BroadcastRead(tenantID int64, guardianAccountID int64, threadID int64, studentID int64, audience string) {
	p.callbacks.BroadcastRead(tenantID, guardianAccountID, threadID, studentID, audience)
}
func (p conversationCoreBinding) LeadershipOption(ctx context.Context) (messaging.LeadershipOption, error) {
	if p.callbacks.LeadershipOption == nil {
		return messaging.LeadershipOption{}, nil
	}
	return p.callbacks.LeadershipOption(ctx)
}
