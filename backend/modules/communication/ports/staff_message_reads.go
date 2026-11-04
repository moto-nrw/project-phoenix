package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

type ParentStaffMessageReads interface {
	MarkReadUpTo(ctx context.Context, tenantID, threadID, accountID int64, readAt time.Time, readMessageID int64) (bool, error)
	ClearUnreadForStaff(ctx context.Context, tenantID, accountID int64, bounds []communication.PortalReadCursorBound) ([]int64, error)
	MarkStaffHandledUpTo(ctx context.Context, tenantID, threadID int64, handledAt time.Time, handledMessageID int64) error
	MarkStaffUnread(ctx context.Context, tenantID, threadID, accountID int64) error
	ClearStaffUnreadMark(ctx context.Context, tenantID, threadID int64, observedAt time.Time) (bool, error)
	UnreadMessageCountForStaff(ctx context.Context, accountID int64, reach communication.ParentMessageStaffReach) (int, error)
	UnreadMessageCountForStaffInGroups(ctx context.Context, accountID int64, reach communication.ParentMessageStaffReach, groupIDs []int64) (int, error)
	ListInboxForStaff(ctx context.Context, accountID int64, reach communication.ParentMessageStaffReach, onlyUnread bool) ([]*communication.PortalInboxThread, error)
	ListThreadsForStudent(ctx context.Context, accountID, studentID int64, reach communication.ParentMessageStaffReach) ([]*communication.PortalInboxThread, error)
	ListThreadsForGuardianStudentWithStaff(ctx context.Context, accountID, studentID int64, staffAccounts map[int64][]int64) ([]*communication.PortalInboxThread, error)
	ListThreadsForGuardianTenantsWithStaff(ctx context.Context, accountID int64, tenantIDs []int64, staffAccounts map[int64][]int64) ([]*communication.PortalInboxThread, error)
	UnreadMessageCountForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64) (int, error)
	FindThreadHeader(ctx context.Context, threadID int64) (*communication.ParentThreadHeaderRecord, error)
	LatestReadCursorByOtherStaff(ctx context.Context, threadID, excludeAccountID int64, staffAccounts map[int64][]int64) (*communication.ParentReadCursorRecord, error)
	GuardianReadCursor(ctx context.Context, threadID int64) (*communication.ParentReadCursorRecord, error)
}
