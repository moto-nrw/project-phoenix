package communication

import (
	"context"
	"time"
)

// Native conversation snapshots retain every metadata and recipient field.
type ParentReadCursorRecord struct {
	LastReadAt        time.Time
	LastReadMessageID int64
}
type ParentThreadHeaderRecord struct {
	StudentName      string
	GuardianName     string
	RelationshipType string
}
type ParentGuardianRecipient struct {
	AccountID        int64
	Name             string
	RelationshipType string
	IsPrimary        bool
	PortalLocale     string
}
type ParentReadCommands interface {
	ParentReadMarkReadUpTo(ctx context.Context, tenantID, threadID, accountID int64, readAt time.Time, readMessageID int64) (bool, error)
	ParentReadClearUnreadForStaff(ctx context.Context, tenantID, accountID int64, bounds []PortalReadCursorBound) ([]int64, error)
	ParentReadMarkStaffHandledUpTo(ctx context.Context, tenantID, threadID int64, handledAt time.Time, handledMessageID int64) error
	ParentReadMarkStaffUnread(ctx context.Context, tenantID, threadID, accountID int64) error
	ParentReadClearStaffUnreadMark(ctx context.Context, tenantID, threadID int64, observedAt time.Time) (bool, error)
}
type ParentReadQueries interface {
	ParentReadUnreadMessageCountForStaff(ctx context.Context, accountID int64, reach ParentMessageStaffReach) (int, error)
	ParentReadUnreadMessageCountForStaffInGroups(ctx context.Context, accountID int64, reach ParentMessageStaffReach, groupIDs []int64) (int, error)
	ParentReadListInboxForStaff(ctx context.Context, accountID int64, reach ParentMessageStaffReach, onlyUnread bool) ([]*PortalInboxThread, error)

	ParentReadListThreadsForStudent(ctx context.Context, accountID, studentID int64, reach ParentMessageStaffReach) ([]*PortalInboxThread, error)

	ParentReadFindThreadHeader(ctx context.Context, threadID int64) (*ParentThreadHeaderRecord, error)
	ParentReadLatestReadCursorByOther(ctx context.Context, threadID, excludeAccountID int64) (*ParentReadCursorRecord, error)
	ParentReadGuardianReadCursor(ctx context.Context, threadID int64) (*ParentReadCursorRecord, error)
}
type ParentReadRecords interface {
	ParentReadCommands
	ParentReadQueries
}
type ParentMessageRecords interface {
	ParentMessageCreate(ctx context.Context, message *PortalParentMessage) error

	ParentMessageListByThread(ctx context.Context, threadID int64, limit int) ([]*PortalParentMessage, error)
	ParentMessageFindEventByRef(ctx context.Context, threadID int64, eventType, refTable string, refID int64) (*PortalParentMessage, error)
}
type ParentThreadRecords interface {
	ParentThreadLockForMessageAppend(ctx context.Context, threadID int64) error
	ParentThreadTouchLastMessage(ctx context.Context, threadID int64, at time.Time, messageID int64, senderKind, body string) error

	ParentThreadFindByID(ctx context.Context, id int64) (*PortalParentMessageThread, error)
	ParentThreadFindByStudentGuardian(ctx context.Context, studentID, guardianAccountID int64, audience string) (*PortalParentMessageThread, error)
	ParentThreadGetOrCreate(ctx context.Context, tenantID, studentID, guardianAccountID int64, audience string) (*PortalParentMessageThread, error)
	ParentThreadListGuardiansForStudent(ctx context.Context, studentID int64) ([]*ParentGuardianRecipient, error)
	ParentThreadListStudentIDsWithMessageableGuardian(ctx context.Context) ([]int64, error)
}
