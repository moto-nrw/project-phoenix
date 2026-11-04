package messaging

import (
	"context"
	"time"

	values "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
)

type InboxThread values.InboxThread
type MessageableGuardian values.MessageableGuardian
type ParentMessage values.ParentMessage
type ParentMessageThread values.ParentMessageThread
type ReadCursor values.ReadCursor
type ReadCursorBound values.ReadCursorBound
type ThreadHeader values.ThreadHeader

// StaffReach is which conversations a staff reader lists and counts.
type StaffReach = values.StaffReach
type ParentMessageReadRepository interface {
	MarkReadUpTo(ctx context.Context, tenantID, threadID, accountID int64, readAt time.Time, readMessageID int64) (bool, error)
	ClearUnreadForStaff(ctx context.Context, tenantID, accountID int64, bounds []ReadCursorBound) ([]int64, error)
	MarkStaffHandledUpTo(ctx context.Context, tenantID, threadID int64, handledAt time.Time, handledMessageID int64) error
	MarkStaffUnread(ctx context.Context, tenantID, threadID, accountID int64) error
	ClearStaffUnreadMark(ctx context.Context, tenantID, threadID int64, observedAt time.Time) (bool, error)
	UnreadMessageCountForStaff(ctx context.Context, accountID int64, reach StaffReach) (int, error)
	UnreadMessageCountForStaffInGroups(ctx context.Context, accountID int64, reach StaffReach, groupIDs []int64) (int, error)
	ListInboxForStaff(ctx context.Context, accountID int64, reach StaffReach, onlyUnread bool) ([]*InboxThread, error)

	ListThreadsForStudent(ctx context.Context, accountID, studentID int64, reach StaffReach) ([]*InboxThread, error)

	FindThreadHeader(ctx context.Context, threadID int64) (*ThreadHeader, error)
	LatestReadCursorByOther(ctx context.Context, threadID, excludeAccountID int64) (*ReadCursor, error)
	GuardianReadCursor(ctx context.Context, threadID int64) (*ReadCursor, error)
}
type ParentMessageRepository interface {
	Create(ctx context.Context, message *ParentMessage) error

	ListByThread(ctx context.Context, threadID int64, limit int) ([]*ParentMessage, error)
	FindEventByRef(ctx context.Context, threadID int64, eventType, refTable string, refID int64) (*ParentMessage, error)
}
type ParentMessageThreadRepository interface {
	LockForMessageAppend(ctx context.Context, threadID int64) error
	TouchLastMessage(ctx context.Context, threadID int64, at time.Time, messageID int64, senderKind, body string) error

	FindByID(ctx context.Context, id int64) (*ParentMessageThread, error)
	FindByStudentGuardian(ctx context.Context, studentID, guardianAccountID int64, audience string) (*ParentMessageThread, error)
	GetOrCreate(ctx context.Context, tenantID, studentID, guardianAccountID int64, audience string) (*ParentMessageThread, error)
	ListGuardiansForStudent(ctx context.Context, studentID int64) ([]*MessageableGuardian, error)
	ListStudentIDsWithMessageableGuardian(ctx context.Context) ([]int64, error)
}

func (v *ParentMessage) GetTenantID() int64         { return v.TenantID }
func (v *ParentMessage) SetTenantID(id int64)       { v.TenantID = id }
func (v *ParentMessageThread) GetTenantID() int64   { return v.TenantID }
func (v *ParentMessageThread) SetTenantID(id int64) { v.TenantID = id }
