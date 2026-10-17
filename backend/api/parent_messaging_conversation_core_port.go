package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

func BindParentConversationCore(source communication.PortalConversationCore) parentportal.ConversationCore {
	if source == nil {
		return nil
	}
	return parentportal.NewConversationCore(parentportal.ConversationCoreCallbacks{AppendMessage: func(ctx context.Context, msg *parentportal.ParentMessage) error {
		ownerMsg := parentMessagingPointer(msg, parentMessagingToOwnerParentMessage)
		result0 := source.PortalAppendMessage(ctx, ownerMsg)
		if msg != nil && ownerMsg != nil {
			*msg = parentMessagingFromOwnerParentMessage(*ownerMsg)
		}
		return result0
	},
		MarkReadToNewest: func(ctx context.Context, tenantID int64, threadID int64, accountID int64, staffReader bool, messages []*parentportal.ParentMessage) (bool, error) {
			ownerMessages := parentMessagingSlice(messages, func(row *parentportal.ParentMessage) *communication.PortalParentMessage {
				return parentMessagingPointer(row, parentMessagingToOwnerParentMessage)
			})
			result0, result1 := source.PortalMarkReadToNewest(ctx, tenantID, threadID, accountID, staffReader, ownerMessages)
			return result0, result1
		},
		DecorateReadReceipts: func(ctx context.Context, threadID int64, otherAccountID int64, messages []*parentportal.ParentMessage) {
			ownerMessages := parentMessagingSlice(messages, func(row *parentportal.ParentMessage) *communication.PortalParentMessage {
				return parentMessagingPointer(row, parentMessagingToOwnerParentMessage)
			})
			source.PortalDecorateReadReceipts(ctx, threadID, otherAccountID, ownerMessages)
			for i, row := range ownerMessages {
				if row == nil {
					messages[i] = nil
				} else if messages[i] == nil {
					messages[i] = parentMessagingPointer(row, parentMessagingFromOwnerParentMessage)
				} else {
					*messages[i] = parentMessagingFromOwnerParentMessage(*row)
				}
			}
		},
		Broadcast: func(tenantID int64, guardianAccountID int64, threadID int64, studentID int64) {
			source.Broadcast(tenantID, guardianAccountID, threadID, studentID)
		},
		BroadcastRead: func(tenantID int64, guardianAccountID int64, threadID int64, studentID int64) {
			source.BroadcastRead(tenantID, guardianAccountID, threadID, studentID)
		},
	})
}
