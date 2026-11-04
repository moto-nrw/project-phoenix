package parentpostgres

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

func (r parentMessageStaffRepository) ParentReadMarkReadUpTo(ctx context.Context, tenantID, threadID, accountID int64, readAt time.Time, readMessageID int64) (bool, error) {
	return r.MarkReadUpTo(ctx, tenantID, threadID, accountID, readAt, readMessageID)
}
func (r parentMessageStaffRepository) ParentReadClearUnreadForStaff(ctx context.Context, tenantID, accountID int64, bounds []communication.PortalReadCursorBound) ([]int64, error) {
	return r.ClearUnreadForStaff(ctx, tenantID, accountID, bounds)
}
func (r parentMessageStaffRepository) ParentReadMarkStaffHandledUpTo(ctx context.Context, tenantID, threadID int64, handledAt time.Time, handledMessageID int64) error {
	return r.MarkStaffHandledUpTo(ctx, tenantID, threadID, handledAt, handledMessageID)
}
func (r parentMessageStaffRepository) ParentReadMarkStaffUnread(ctx context.Context, tenantID, threadID, accountID int64) error {
	return r.MarkStaffUnread(ctx, tenantID, threadID, accountID)
}
func (r parentMessageStaffRepository) ParentReadClearStaffUnreadMark(ctx context.Context, tenantID, threadID int64, observedAt time.Time) (bool, error) {
	return r.ClearStaffUnreadMark(ctx, tenantID, threadID, observedAt)
}
func (r parentMessageStaffRepository) ParentReadUnreadMessageCountForStaff(ctx context.Context, accountID int64, reach communication.ParentMessageStaffReach) (int, error) {
	return r.UnreadMessageCountForStaff(ctx, accountID, reach)
}
func (r parentMessageStaffRepository) ParentReadUnreadMessageCountForStaffInGroups(ctx context.Context, accountID int64, reach communication.ParentMessageStaffReach, groupIDs []int64) (int, error) {
	return r.UnreadMessageCountForStaffInGroups(ctx, accountID, reach, groupIDs)
}
func (r parentMessageStaffRepository) ParentReadListInboxForStaff(ctx context.Context, accountID int64, reach communication.ParentMessageStaffReach, onlyUnread bool) ([]*communication.PortalInboxThread, error) {
	return r.ListInboxForStaff(ctx, accountID, reach, onlyUnread)
}
func (r parentMessageStaffRepository) ParentReadListThreadsForStudent(ctx context.Context, accountID, studentID int64, reach communication.ParentMessageStaffReach) ([]*communication.PortalInboxThread, error) {
	return r.ListThreadsForStudent(ctx, accountID, studentID, reach)
}
func (r parentMessageStaffRepository) ParentReadFindThreadHeader(ctx context.Context, threadID int64) (*communication.ParentThreadHeaderRecord, error) {
	return r.FindThreadHeader(ctx, threadID)
}
func (r parentMessageStaffRepository) ParentReadLatestReadCursorByOther(ctx context.Context, threadID, excludeAccountID int64) (*communication.ParentReadCursorRecord, error) {
	return r.LatestReadCursorByOther(ctx, threadID, excludeAccountID)
}
func (r parentMessageStaffRepository) ParentReadGuardianReadCursor(ctx context.Context, threadID int64) (*communication.ParentReadCursorRecord, error) {
	return r.GuardianReadCursor(ctx, threadID)
}
func (r schoolParentMessageReadRepository) ParentReadListInboxForStaff(ctx context.Context, accountID int64, reach communication.ParentMessageStaffReach, onlyUnread bool) ([]*communication.PortalInboxThread, error) {
	return r.ListInboxForStaff(ctx, accountID, reach, onlyUnread)
}
func (r schoolParentMessageReadRepository) ParentReadListThreadsForStudent(ctx context.Context, accountID, studentID int64, reach communication.ParentMessageStaffReach) ([]*communication.PortalInboxThread, error) {
	return r.ListThreadsForStudent(ctx, accountID, studentID, reach)
}
func (r groupParentMessageReadRepository) ParentReadListInboxForStaff(ctx context.Context, accountID int64, reach communication.ParentMessageStaffReach, onlyUnread bool) ([]*communication.PortalInboxThread, error) {
	return r.ListInboxForStaff(ctx, accountID, reach, onlyUnread)
}
func (r groupParentMessageReadRepository) ParentReadListThreadsForStudent(ctx context.Context, accountID, studentID int64, reach communication.ParentMessageStaffReach) ([]*communication.PortalInboxThread, error) {
	return r.ListThreadsForStudent(ctx, accountID, studentID, reach)
}

func (r *messageRepository) ParentMessageCreate(ctx context.Context, message *communication.PortalParentMessage) error {
	return r.Create(ctx, message)
}
func (r *messageRepository) ParentMessageListByThread(ctx context.Context, threadID int64, limit int) ([]*communication.PortalParentMessage, error) {
	return r.ListByThread(ctx, threadID, limit)
}
func (r *messageRepository) ParentMessageFindEventByRef(ctx context.Context, threadID int64, eventType, refTable string, refID int64) (*communication.PortalParentMessage, error) {
	return r.FindEventByRef(ctx, threadID, eventType, refTable, refID)
}
func (r *threadRepository) ParentThreadLockForMessageAppend(ctx context.Context, threadID int64) error {
	return r.LockForMessageAppend(ctx, threadID)
}
func (r *threadRepository) ParentThreadTouchLastMessage(ctx context.Context, threadID int64, at time.Time, messageID int64, senderKind, body string) error {
	return r.TouchLastMessage(ctx, threadID, at, messageID, senderKind, body)
}
func (r *threadRepository) ParentThreadFindByID(ctx context.Context, id int64) (*communication.PortalParentMessageThread, error) {
	return r.FindByID(ctx, id)
}
func (r *threadRepository) ParentThreadFindByStudentGuardian(ctx context.Context, studentID, guardianAccountID int64, audience string) (*communication.PortalParentMessageThread, error) {
	return r.FindByStudentGuardian(ctx, studentID, guardianAccountID, audience)
}
func (r *threadRepository) ParentThreadGetOrCreate(ctx context.Context, tenantID, studentID, guardianAccountID int64, audience string) (*communication.PortalParentMessageThread, error) {
	return r.GetOrCreate(ctx, tenantID, studentID, guardianAccountID, audience)
}
func (r *threadRepository) ParentThreadListGuardiansForStudent(ctx context.Context, studentID int64) ([]*communication.ParentGuardianRecipient, error) {
	return r.ListGuardiansForStudent(ctx, studentID)
}
func (r *threadRepository) ParentThreadListStudentIDsWithMessageableGuardian(ctx context.Context) ([]int64, error) {
	return r.ListStudentIDsWithMessageableGuardian(ctx)
}
