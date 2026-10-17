package communication

import (
	"context"
)

type PortalMessageInboxReads interface {
	PortalListThreadsForGuardianStudent(ctx context.Context, accountID int64, studentID int64) ([]*PortalInboxThread, error)
	PortalListThreadsForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64) ([]*PortalInboxThread, error)
	UnreadMessageCountForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64) (int, error)
}
