package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

func BindParentMessageInboxReads(source communication.PortalMessageInboxReads) parentportal.MessageInboxReads {
	if source == nil {
		return nil
	}
	return parentportal.NewMessageInboxReads(parentportal.MessageInboxReadsCallbacks{ListThreadsForGuardianStudent: func(ctx context.Context, accountID int64, studentID int64) ([]*parentportal.InboxThread, error) {
		result0, result1 := source.PortalListThreadsForGuardianStudent(ctx, accountID, studentID)
		return ParentMessagingSlice(result0, func(row *communication.PortalInboxThread) *parentportal.InboxThread {
			return ParentMessagingPointer(row, parentMessagingFromOwnerInboxThread)
		}), result1
	},
		ListThreadsForGuardianTenants: func(ctx context.Context, accountID int64, tenantIDs []int64) ([]*parentportal.InboxThread, error) {
			result0, result1 := source.PortalListThreadsForGuardianTenants(ctx, accountID, tenantIDs)
			return ParentMessagingSlice(result0, func(row *communication.PortalInboxThread) *parentportal.InboxThread {
				return ParentMessagingPointer(row, parentMessagingFromOwnerInboxThread)
			}), result1
		},
		UnreadMessageCountForGuardianTenants: func(ctx context.Context, accountID int64, tenantIDs []int64) (int, error) {
			result0, result1 := source.UnreadMessageCountForGuardianTenants(ctx, accountID, tenantIDs)
			return result0, result1
		},
	})
}
