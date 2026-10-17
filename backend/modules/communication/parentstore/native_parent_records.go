package parentstore

import (
	"context"
	"time"

	models "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/communication"
)

// Native facets call the same bound legacy methods until the store cutover.

func parentThreadToRecord(v *models.ParentMessageThread) *communication.PortalParentMessageThread {
	if v == nil {
		return nil
	}
	row := portalFromRetainedParentMessageThread(*v)
	return &row
}
func parentBoundsFromRecords(rows []communication.PortalReadCursorBound) []models.ReadCursorBound {
	if rows == nil {
		return nil
	}
	out := make([]models.ReadCursorBound, len(rows))
	for i, row := range rows {
		out[i] = models.ReadCursorBound(row)
	}
	return out
}
func (r *messageRepository) ParentMessageCreate(ctx context.Context, message *communication.PortalParentMessage) error {
	value, mapErr := portalToRetainedParentMessagePointer(message)
	if mapErr != nil {
		return mapErr
	}
	err := r.Create(ctx, value)
	mapped, mapErr := portalFromRetainedParentMessagePointer(value)
	if message != nil && mapped != nil {
		*message = *mapped
	}
	if err != nil {
		return err
	}
	return mapErr
}

func (r *messageRepository) ParentMessageListByThread(ctx context.Context, threadID int64, limit int) ([]*communication.PortalParentMessage, error) {
	rows, err := r.ListByThread(ctx, threadID, limit)
	mapped, mapErr := portalFromRetainedParentMessageSlice(rows)
	if err != nil {
		return mapped, err
	}
	return mapped, mapErr
}
func (r *messageRepository) ParentMessageFindEventByRef(ctx context.Context, threadID int64, eventType, refTable string, refID int64) (*communication.PortalParentMessage, error) {
	rows, err := r.FindEventByRef(ctx, threadID, eventType, refTable, refID)
	mapped, mapErr := portalFromRetainedParentMessagePointer(rows)
	if err != nil {
		return mapped, err
	}
	return mapped, mapErr
}

func (r *threadRepository) ParentThreadLockForMessageAppend(ctx context.Context, threadID int64) error {
	return r.LockForMessageAppend(ctx, threadID)
}
func (r *threadRepository) ParentThreadTouchLastMessage(ctx context.Context, threadID int64, at time.Time, messageID int64, senderKind, body string) error {
	return r.TouchLastMessage(ctx, threadID, at, messageID, senderKind, body)
}

func (r *threadRepository) ParentThreadFindByID(ctx context.Context, id int64) (*communication.PortalParentMessageThread, error) {
	rows, err := r.FindByID(ctx, id)
	return parentThreadToRecord(rows), err
}
func (r *threadRepository) ParentThreadFindByStudentGuardian(ctx context.Context, studentID, guardianAccountID int64) (*communication.PortalParentMessageThread, error) {
	rows, err := r.FindByStudentGuardian(ctx, studentID, guardianAccountID)
	return parentThreadToRecord(rows), err
}
func (r *threadRepository) ParentThreadGetOrCreate(ctx context.Context, tenantID, studentID, guardianAccountID int64) (*communication.PortalParentMessageThread, error) {
	rows, err := r.GetOrCreate(ctx, tenantID, studentID, guardianAccountID)
	return parentThreadToRecord(rows), err
}
func (r *threadRepository) ParentThreadListGuardiansForStudent(ctx context.Context, studentID int64) ([]*communication.ParentGuardianRecipient, error) {
	rows, err := r.ListGuardiansForStudent(ctx, studentID)
	if rows == nil {
		return nil, err
	}
	mapped := make([]*communication.ParentGuardianRecipient, len(rows))
	for i, row := range rows {
		mapped[i] = (*communication.ParentGuardianRecipient)(row)
	}
	return mapped, err
}
func (r parentMessageStaffRepository) ParentReadMarkReadUpTo(ctx context.Context, tenantID, threadID, accountID int64, readAt time.Time, readMessageID int64) (bool, error) {
	return r.MarkReadUpTo(ctx, tenantID, threadID, accountID, readAt, readMessageID)
}
func (r parentMessageStaffRepository) ParentReadClearUnreadForStaff(ctx context.Context, tenantID, accountID int64, bounds []communication.PortalReadCursorBound) ([]int64, error) {
	return r.ClearUnreadForStaff(ctx, tenantID, accountID, parentBoundsFromRecords(bounds))
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
func (r parentMessageStaffRepository) ParentReadUnreadMessageCountForStaff(ctx context.Context, accountID int64, allStudents bool) (int, error) {
	return r.UnreadMessageCountForStaff(ctx, accountID, allStudents)
}
func (r parentMessageStaffRepository) ParentReadUnreadMessageCountForStaffInGroups(ctx context.Context, accountID int64, allStudents bool, groupIDs []int64) (int, error) {
	return r.UnreadMessageCountForStaffInGroups(ctx, accountID, allStudents, groupIDs)
}
func (r parentMessageStaffRepository) ParentReadListInboxForStaff(ctx context.Context, accountID int64, allStudents bool, onlyUnread bool) ([]*communication.PortalInboxThread, error) {
	rows, err := r.ListInboxForStaff(ctx, accountID, allStudents, onlyUnread)
	mapped, mapErr := portalFromRetainedInboxThreadSlice(rows)
	if err != nil {
		return mapped, err
	}
	return mapped, mapErr
}

func (r parentMessageStaffRepository) ParentReadListThreadsForStudent(ctx context.Context, accountID, studentID int64) ([]*communication.PortalInboxThread, error) {
	rows, err := r.ListThreadsForStudent(ctx, accountID, studentID)
	mapped, mapErr := portalFromRetainedInboxThreadSlice(rows)
	if err != nil {
		return mapped, err
	}
	return mapped, mapErr
}

func (r parentMessageStaffRepository) ParentReadFindThreadHeader(ctx context.Context, threadID int64) (*communication.ParentThreadHeaderRecord, error) {
	rows, err := r.FindThreadHeader(ctx, threadID)
	return (*communication.ParentThreadHeaderRecord)(rows), err
}
func (r parentMessageStaffRepository) ParentReadLatestReadCursorByOther(ctx context.Context, threadID, excludeAccountID int64) (*communication.ParentReadCursorRecord, error) {
	rows, err := r.LatestReadCursorByOther(ctx, threadID, excludeAccountID)
	return (*communication.ParentReadCursorRecord)(rows), err
}
func (r parentMessageStaffRepository) ParentReadGuardianReadCursor(ctx context.Context, threadID int64) (*communication.ParentReadCursorRecord, error) {
	rows, err := r.GuardianReadCursor(ctx, threadID)
	return (*communication.ParentReadCursorRecord)(rows), err
}
func (r schoolParentMessageReadRepository) ParentReadListInboxForStaff(ctx context.Context, accountID int64, allStudents bool, onlyUnread bool) ([]*communication.PortalInboxThread, error) {
	rows, err := r.ListInboxForStaff(ctx, accountID, allStudents, onlyUnread)
	mapped, mapErr := portalFromRetainedInboxThreadSlice(rows)
	if err != nil {
		return mapped, err
	}
	return mapped, mapErr
}

func (r schoolParentMessageReadRepository) ParentReadListThreadsForStudent(ctx context.Context, accountID, studentID int64) ([]*communication.PortalInboxThread, error) {
	rows, err := r.ListThreadsForStudent(ctx, accountID, studentID)
	mapped, mapErr := portalFromRetainedInboxThreadSlice(rows)
	if err != nil {
		return mapped, err
	}
	return mapped, mapErr
}
func (r groupParentMessageReadRepository) ParentReadListInboxForStaff(ctx context.Context, accountID int64, allStudents bool, onlyUnread bool) ([]*communication.PortalInboxThread, error) {
	rows, err := r.ListInboxForStaff(ctx, accountID, allStudents, onlyUnread)
	mapped, mapErr := portalFromRetainedInboxThreadSlice(rows)
	if err != nil {
		return mapped, err
	}
	return mapped, mapErr
}

func (r groupParentMessageReadRepository) ParentReadListThreadsForStudent(ctx context.Context, accountID, studentID int64) ([]*communication.PortalInboxThread, error) {
	rows, err := r.ListThreadsForStudent(ctx, accountID, studentID)
	mapped, mapErr := portalFromRetainedInboxThreadSlice(rows)
	if err != nil {
		return mapped, err
	}
	return mapped, mapErr
}
