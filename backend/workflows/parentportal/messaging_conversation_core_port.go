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
	Broadcast            func(tenantID int64, guardianAccountID int64, threadID int64, studentID int64)
	BroadcastRead        func(tenantID int64, guardianAccountID int64, threadID int64, studentID int64)
}

func NewConversationCore(c ConversationCoreCallbacks) ConversationCore {
	return conversationCoreBinding{c}
}

type conversationCoreBinding struct{ callbacks ConversationCoreCallbacks }

func (p conversationCoreBinding) AppendMessage(ctx context.Context, msg *messaging.ParentMessage) error {
	nativeMsg := messagingValuePointer(msg, messagingFromAppParentMessage)
	result0 := p.callbacks.AppendMessage(ctx, nativeMsg)
	if msg != nil && nativeMsg != nil {
		*msg = messagingToAppParentMessage(*nativeMsg)
	}
	return result0
}
func (p conversationCoreBinding) MarkReadToNewest(ctx context.Context, tenantID int64, threadID int64, accountID int64, staffReader bool, messages []*messaging.ParentMessage) (bool, error) {
	nativeMessages := messagingValueSlice(messages, func(row *messaging.ParentMessage) *ParentMessage {
		return messagingValuePointer(row, messagingFromAppParentMessage)
	})
	result0, result1 := p.callbacks.MarkReadToNewest(ctx, tenantID, threadID, accountID, staffReader, nativeMessages)
	return result0, result1
}
func (p conversationCoreBinding) DecorateReadReceipts(ctx context.Context, threadID int64, otherAccountID int64, messages []*messaging.ParentMessage) {
	nativeMessages := messagingValueSlice(messages, func(row *messaging.ParentMessage) *ParentMessage {
		return messagingValuePointer(row, messagingFromAppParentMessage)
	})
	p.callbacks.DecorateReadReceipts(ctx, threadID, otherAccountID, nativeMessages)
	for i, row := range nativeMessages {
		if row == nil {
			messages[i] = nil
		} else if messages[i] == nil {
			messages[i] = messagingValuePointer(row, messagingToAppParentMessage)
		} else {
			*messages[i] = messagingToAppParentMessage(*row)
		}
	}
}
func (p conversationCoreBinding) Broadcast(tenantID int64, guardianAccountID int64, threadID int64, studentID int64) {
	p.callbacks.Broadcast(tenantID, guardianAccountID, threadID, studentID)
}
func (p conversationCoreBinding) BroadcastRead(tenantID int64, guardianAccountID int64, threadID int64, studentID int64) {
	p.callbacks.BroadcastRead(tenantID, guardianAccountID, threadID, studentID)
}
