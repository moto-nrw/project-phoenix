package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

func BindParentMessageThreadAccess(source communication.PortalMessageThreadAccess) parentportal.MessageThreadAccess {
	if source == nil {
		return nil
	}
	return parentportal.NewMessageThreadAccess(parentportal.MessageThreadAccessCallbacks{FindByStudentGuardian: func(ctx context.Context, studentID int64, guardianAccountID int64, audience string) (*parentportal.ParentMessageThread, error) {
		result0, result1 := source.PortalFindByStudentGuardian(ctx, studentID, guardianAccountID, audience)
		return ParentMessagingPointer(result0, parentMessagingFromOwnerParentMessageThread), result1
	},
		GetOrCreate: func(ctx context.Context, tenantID int64, studentID int64, guardianAccountID int64, audience string) (*parentportal.ParentMessageThread, error) {
			result0, result1 := source.PortalGetOrCreate(ctx, tenantID, studentID, guardianAccountID, audience)
			return ParentMessagingPointer(result0, parentMessagingFromOwnerParentMessageThread), result1
		},
	})
}
