package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

type ParentInboxReads interface {
	ListInboxForStaff(ctx context.Context, accountID int64, allStudents, onlyUnread bool) ([]*communication.PortalInboxThread, error)
	ListThreadsForStudent(ctx context.Context, accountID, studentID int64) ([]*communication.PortalInboxThread, error)
	ListThreadsForGuardianStudentWithStaff(ctx context.Context, accountID, studentID int64, staffAccounts map[int64][]int64) ([]*communication.PortalInboxThread, error)
	ListThreadsForGuardianTenantsWithStaff(ctx context.Context, accountID int64, tenantIDs []int64, staffAccounts map[int64][]int64) ([]*communication.PortalInboxThread, error)
	UnreadMessageCountForStaff(ctx context.Context, accountID int64, allStudents bool) (int, error)
	UnreadMessageCountForStaffInGroups(ctx context.Context, accountID int64, allStudents bool, groupIDs []int64) (int, error)
	UnreadMessageCountForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64) (int, error)
	FindThreadHeader(ctx context.Context, threadID int64) (*communication.ParentThreadHeaderRecord, error)
	LatestReadCursorByOtherStaff(ctx context.Context, threadID, excludeAccountID int64, staffAccounts map[int64][]int64) (*communication.ParentReadCursorRecord, error)
	GuardianReadCursor(ctx context.Context, threadID int64) (*communication.ParentReadCursorRecord, error)
}
