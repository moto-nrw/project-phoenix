package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

func BindParentMessageReads(source communication.PortalMessageReads) parentportal.MessageReads {
	if source == nil {
		return nil
	}
	return parentportal.NewMessageReads(parentportal.MessageReadsCallbacks{ListByThread: func(ctx context.Context, threadID int64, limit int) ([]*parentportal.ParentMessage, error) {
		result0, result1 := source.PortalListByThread(ctx, threadID, limit)
		return ParentMessagingSlice(result0, func(row *communication.PortalParentMessage) *parentportal.ParentMessage {
			return ParentMessagingPointer(row, ParentMessagingFromOwnerParentMessage)
		}), result1
	},
	})
}
