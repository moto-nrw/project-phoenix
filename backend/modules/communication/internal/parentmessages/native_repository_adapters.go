package messaging

import (
	"context"
	"time"

	values "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
)

func NewMessageRepository(source values.ParentMessageRepositorySource) ParentMessageRepository {
	if source == nil {
		return nil
	}
	return &nativeParentMessageRepository{source: source}
}

type nativeParentMessageRepository struct {
	source values.ParentMessageRepositorySource
}

func (r *nativeParentMessageRepository) Create(ctx context.Context, message *ParentMessage) error {
	record, mapErr := values.ParentMessageToRecord((*values.ParentMessage)(message))
	if mapErr != nil {
		return mapErr
	}
	payloadBefore := values.SnapshotParentMessagePayload(record)
	err := r.source.ParentMessageCreate(ctx, record)
	mapped, mapErr := values.ParentMessageFromRecord(record)
	if message != nil && mapped != nil {
		originalPayload := message.Payload
		*message = ParentMessage(*mapped)
		if values.SameParentRecordPayload(payloadBefore, record.Payload) {
			message.Payload = originalPayload
		}
	}
	if err != nil {
		return err
	}
	return mapErr
}

func (r *nativeParentMessageRepository) ListByThread(ctx context.Context, threadID int64, limit int) ([]*ParentMessage, error) {
	rows, err := r.source.ParentMessageListByThread(ctx, threadID, limit)
	if rows == nil {
		return nil, err
	}
	mapped := make([]*ParentMessage, len(rows))
	for i, row := range rows {
		value, mapErr := values.ParentMessageFromRecord(row)
		if mapErr != nil {
			if err != nil {
				return nil, err
			}
			return nil, mapErr
		}
		mapped[i] = (*ParentMessage)(value)
	}
	return mapped, err
}
func (r *nativeParentMessageRepository) FindEventByRef(ctx context.Context, threadID int64, eventType, refTable string, refID int64) (*ParentMessage, error) {
	row, err := r.source.ParentMessageFindEventByRef(ctx, threadID, eventType, refTable, refID)
	mapped, mapErr := values.ParentMessageFromRecord(row)
	if err != nil {
		return (*ParentMessage)(mapped), err
	}
	return (*ParentMessage)(mapped), mapErr
}
func NewThreadRepository(source values.ParentMessageThreadRepositorySource) ParentMessageThreadRepository {
	if source == nil {
		return nil
	}
	return &nativeParentThreadRepository{source: source}
}

type nativeParentThreadRepository struct {
	source values.ParentMessageThreadRepositorySource
}

func (r *nativeParentThreadRepository) LockForMessageAppend(ctx context.Context, threadID int64) error {
	return r.source.ParentThreadLockForMessageAppend(ctx, threadID)
}
func (r *nativeParentThreadRepository) TouchLastMessage(ctx context.Context, threadID int64, at time.Time, messageID int64, senderKind, body string) error {
	return r.source.ParentThreadTouchLastMessage(ctx, threadID, at, messageID, senderKind, body)
}

func (r *nativeParentThreadRepository) FindByID(ctx context.Context, id int64) (*ParentMessageThread, error) {
	row, err := r.source.ParentThreadFindByID(ctx, id)
	return (*ParentMessageThread)(values.ParentMessageThreadFromRecord(row)), err
}
func (r *nativeParentThreadRepository) FindByStudentGuardian(ctx context.Context, studentID, guardianAccountID int64) (*ParentMessageThread, error) {
	row, err := r.source.ParentThreadFindByStudentGuardian(ctx, studentID, guardianAccountID)
	return (*ParentMessageThread)(values.ParentMessageThreadFromRecord(row)), err
}
func (r *nativeParentThreadRepository) GetOrCreate(ctx context.Context, tenantID, studentID, guardianAccountID int64) (*ParentMessageThread, error) {
	row, err := r.source.ParentThreadGetOrCreate(ctx, tenantID, studentID, guardianAccountID)
	return (*ParentMessageThread)(values.ParentMessageThreadFromRecord(row)), err
}
func (r *nativeParentThreadRepository) ListGuardiansForStudent(ctx context.Context, studentID int64) ([]*MessageableGuardian, error) {
	rows, err := r.source.ParentThreadListGuardiansForStudent(ctx, studentID)
	if rows == nil {
		return nil, err
	}
	mapped := make([]*MessageableGuardian, len(rows))
	for i, row := range rows {
		mapped[i] = (*MessageableGuardian)(values.MessageableGuardianFromRecord(row))
	}
	return mapped, err
}
func (r *nativeParentThreadRepository) ListStudentIDsWithMessageableGuardian(ctx context.Context) ([]int64, error) {
	return r.source.ParentThreadListStudentIDsWithMessageableGuardian(ctx)
}
func NewReadRepository(source values.ParentMessageReadRepositorySource) ParentMessageReadRepository {
	if source == nil {
		return nil
	}
	return &nativeParentReadRepository{source: source}
}

type nativeParentReadRepository struct {
	source values.ParentMessageReadRepositorySource
}

func (r *nativeParentReadRepository) MarkReadUpTo(ctx context.Context, tenantID, threadID, accountID int64, readAt time.Time, readMessageID int64) (bool, error) {
	return r.source.ParentReadMarkReadUpTo(ctx, tenantID, threadID, accountID, readAt, readMessageID)
}
func (r *nativeParentReadRepository) ClearUnreadForStaff(ctx context.Context, tenantID, accountID int64, bounds []ReadCursorBound) ([]int64, error) {
	var nativeBounds []values.ReadCursorBound
	if bounds != nil {
		nativeBounds = make([]values.ReadCursorBound, len(bounds))
		for i, row := range bounds {
			nativeBounds[i] = values.ReadCursorBound(row)
		}
	}
	return r.source.ParentReadClearUnreadForStaff(ctx, tenantID, accountID, values.ParentReadBoundsToRecords(nativeBounds))
}
func (r *nativeParentReadRepository) MarkStaffHandledUpTo(ctx context.Context, tenantID, threadID int64, handledAt time.Time, handledMessageID int64) error {
	return r.source.ParentReadMarkStaffHandledUpTo(ctx, tenantID, threadID, handledAt, handledMessageID)
}
func (r *nativeParentReadRepository) MarkStaffUnread(ctx context.Context, tenantID, threadID, accountID int64) error {
	return r.source.ParentReadMarkStaffUnread(ctx, tenantID, threadID, accountID)
}
func (r *nativeParentReadRepository) ClearStaffUnreadMark(ctx context.Context, tenantID, threadID int64, observedAt time.Time) (bool, error) {
	return r.source.ParentReadClearStaffUnreadMark(ctx, tenantID, threadID, observedAt)
}
func (r *nativeParentReadRepository) UnreadMessageCountForStaff(ctx context.Context, accountID int64, allStudents bool) (int, error) {
	return r.source.ParentReadUnreadMessageCountForStaff(ctx, accountID, allStudents)
}
func (r *nativeParentReadRepository) UnreadMessageCountForStaffInGroups(ctx context.Context, accountID int64, allStudents bool, groupIDs []int64) (int, error) {
	return r.source.ParentReadUnreadMessageCountForStaffInGroups(ctx, accountID, allStudents, groupIDs)
}
func (r *nativeParentReadRepository) ListInboxForStaff(ctx context.Context, accountID int64, allStudents bool, onlyUnread bool) ([]*InboxThread, error) {
	rows, err := r.source.ParentReadListInboxForStaff(ctx, accountID, allStudents, onlyUnread)
	if rows == nil {
		return nil, err
	}
	mapped := make([]*InboxThread, len(rows))
	for i, row := range rows {
		value, mapErr := values.InboxThreadFromRecord(row)
		if mapErr != nil {
			if err != nil {
				return nil, err
			}
			return nil, mapErr
		}
		mapped[i] = (*InboxThread)(value)
	}
	return mapped, err
}

func (r *nativeParentReadRepository) ListThreadsForStudent(ctx context.Context, accountID, studentID int64) ([]*InboxThread, error) {
	rows, err := r.source.ParentReadListThreadsForStudent(ctx, accountID, studentID)
	if rows == nil {
		return nil, err
	}
	mapped := make([]*InboxThread, len(rows))
	for i, row := range rows {
		value, mapErr := values.InboxThreadFromRecord(row)
		if mapErr != nil {
			if err != nil {
				return nil, err
			}
			return nil, mapErr
		}
		mapped[i] = (*InboxThread)(value)
	}
	return mapped, err
}

func (r *nativeParentReadRepository) FindThreadHeader(ctx context.Context, threadID int64) (*ThreadHeader, error) {
	row, err := r.source.ParentReadFindThreadHeader(ctx, threadID)
	return (*ThreadHeader)(values.ThreadHeaderFromRecord(row)), err
}
func (r *nativeParentReadRepository) LatestReadCursorByOther(ctx context.Context, threadID, excludeAccountID int64) (*ReadCursor, error) {
	row, err := r.source.ParentReadLatestReadCursorByOther(ctx, threadID, excludeAccountID)
	return (*ReadCursor)(values.ReadCursorFromRecord(row)), err
}
func (r *nativeParentReadRepository) GuardianReadCursor(ctx context.Context, threadID int64) (*ReadCursor, error) {
	row, err := r.source.ParentReadGuardianReadCursor(ctx, threadID)
	return (*ReadCursor)(values.ReadCursorFromRecord(row)), err
}
