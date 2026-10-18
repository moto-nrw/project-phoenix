package parentstore

import (
	"context"

	parentinbox "github.com/moto-nrw/project-phoenix/modules/parentmessageinbox/contract"
)

type ParentInbox interface {
	ListInboxForStaff(ctx context.Context, accountID int64, allStudents, onlyUnread bool) ([]*parentinbox.ParentInboxThread, error)
	ListThreadsForStudent(ctx context.Context, accountID, studentID int64) ([]*parentinbox.ParentInboxThread, error)
	ListThreadsForGuardianStudent(ctx context.Context, accountID, studentID int64, staffAccounts map[int64][]int64) ([]*parentinbox.ParentInboxThread, error)
	ListThreadsForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64, staffAccounts map[int64][]int64) ([]*parentinbox.ParentInboxThread, error)
	UnreadMessageCountForStaff(ctx context.Context, accountID int64, allStudents bool) (int, error)
	UnreadMessageCountForStaffInGroups(ctx context.Context, accountID int64, allStudents bool, groupIDs []int64) (int, error)
	UnreadMessageCountForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64) (int, error)
	FindThreadHeader(ctx context.Context, threadID int64) (*parentinbox.ParentThreadHeader, error)
	LatestReadCursorByOtherStaff(ctx context.Context, threadID, excludeAccountID int64, staffAccounts map[int64][]int64) (*parentinbox.ParentReadCursor, error)
	GuardianReadCursor(ctx context.Context, threadID int64) (*parentinbox.ParentReadCursor, error)
}
