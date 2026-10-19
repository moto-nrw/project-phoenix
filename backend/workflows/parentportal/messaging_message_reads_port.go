package parentportal

import (
	"context"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/messaging"
)

type MessageReads interface{ messaging.MessageReads }
type MessageReadsCallbacks struct {
	ListByThread func(ctx context.Context, threadID int64, limit int) ([]*ParentMessage, error)
}

func NewMessageReads(c MessageReadsCallbacks) MessageReads { return messageReadsBinding{c} }

type messageReadsBinding struct{ callbacks MessageReadsCallbacks }

func (p messageReadsBinding) ListByThread(ctx context.Context, threadID int64, limit int) ([]*messaging.ParentMessage, error) {
	result0, result1 := p.callbacks.ListByThread(ctx, threadID, limit)
	return messagingValueSlice(result0, func(row *ParentMessage) *messaging.ParentMessage {
		return messagingValuePointer(row, messagingToAppParentMessage)
	}), result1
}
