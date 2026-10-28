package staffmessaging

import (
	"context"
	"time"

	values "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
)

// These workflow-owned record types are definitions, not Go aliases.
type StaffMessage values.StaffMessage
type StaffMessageThread values.StaffMessageThread
type StaffInboxThread values.StaffInboxThread
type MessageableStaff values.MessageableStaff

type StaffMessageThreadRepository interface {
	GetOrCreateDirect(context.Context, int64, int64) (*StaffMessageThread, error)
	FindByID(context.Context, int64) (*StaffMessageThread, error)
	LockForMessageAppend(context.Context, int64) error
	TouchLastMessage(context.Context, int64, time.Time, int64, int64, string) error
	ParticipantAccountIDs(context.Context, int64) ([]int64, error)
	IsParticipant(context.Context, int64, int64) (bool, error)
	DeleteEmpty(context.Context, time.Time) (int64, error)
}
type StaffMessageRepository interface {
	Create(context.Context, *StaffMessage) error
	ListByThread(context.Context, int64) ([]*StaffMessage, error)
	DeleteOlderThan(context.Context, time.Time) (int64, error)
}
type StaffMessageReadRepository interface {
	MarkReadUpTo(context.Context, int64, int64, time.Time, int64) error
	UnreadCount(context.Context, int64) (int, error)
	ListInbox(context.Context, int64, bool) ([]*StaffInboxThread, error)
	ListMessageableStaff(context.Context, int64) ([]*MessageableStaff, error)
	IsMessageableStaff(context.Context, int64) (bool, error)
	StaffRoleKinds(context.Context, []int64) (map[int64]string, error)
}

type staffThreadRecords struct{ source values.StaffThreadSource }

func NewThreadRepository(source values.StaffThreadSource) StaffMessageThreadRepository {
	if source == nil {
		return nil
	}
	return staffThreadRecords{source: source}
}
func (r staffThreadRecords) GetOrCreateDirect(ctx context.Context, a, b int64) (*StaffMessageThread, error) {
	row, err := r.source.GetOrCreateStaffThread(ctx, a, b)
	return (*StaffMessageThread)(values.StaffThreadFromRecord(row)), err
}
func (r staffThreadRecords) FindByID(ctx context.Context, id int64) (*StaffMessageThread, error) {
	row, err := r.source.FindStaffThread(ctx, id)
	return (*StaffMessageThread)(values.StaffThreadFromRecord(row)), err
}
func (r staffThreadRecords) LockForMessageAppend(ctx context.Context, id int64) error {
	return r.source.LockStaffThreadForAppend(ctx, id)
}
func (r staffThreadRecords) TouchLastMessage(ctx context.Context, id int64, at time.Time, messageID, senderID int64, body string) error {
	return r.source.AdvanceStaffThreadPreview(ctx, id, at, messageID, senderID, body)
}
func (r staffThreadRecords) ParticipantAccountIDs(ctx context.Context, id int64) ([]int64, error) {
	return r.source.StaffParticipantAccountIDs(ctx, id)
}
func (r staffThreadRecords) IsParticipant(ctx context.Context, id, accountID int64) (bool, error) {
	return r.source.HasStaffParticipant(ctx, id, accountID)
}
func (r staffThreadRecords) DeleteEmpty(ctx context.Context, before time.Time) (int64, error) {
	return r.source.DeleteEmptyStaffThreads(ctx, before)
}

type staffMessageRecords struct{ source values.StaffMessageSource }

func NewMessageRepository(source values.StaffMessageSource) StaffMessageRepository {
	if source == nil {
		return nil
	}
	return staffMessageRecords{source: source}
}
func (r staffMessageRecords) Create(ctx context.Context, row *StaffMessage) error {
	return r.source.CreateStaffMessage(ctx, values.StaffMessageToRecord((*values.StaffMessage)(row)))
}
func (r staffMessageRecords) ListByThread(ctx context.Context, id int64) ([]*StaffMessage, error) {
	rows, err := r.source.ListStaffMessages(ctx, id)
	if rows == nil {
		return nil, err
	}
	result := make([]*StaffMessage, len(rows))
	for i, row := range rows {
		result[i] = (*StaffMessage)(values.StaffMessageFromRecord(row))
	}
	return result, err
}
func (r staffMessageRecords) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	return r.source.DeleteStaffMessagesBefore(ctx, before)
}

type staffReadRecords struct{ source values.StaffReadSource }

func NewReadRepository(source values.StaffReadSource) StaffMessageReadRepository {
	if source == nil {
		return nil
	}
	return staffReadRecords{source: source}
}
func (r staffReadRecords) MarkReadUpTo(ctx context.Context, id, accountID int64, at time.Time, messageID int64) error {
	return r.source.MarkStaffReadUpTo(ctx, id, accountID, at, messageID)
}
func (r staffReadRecords) UnreadCount(ctx context.Context, id int64) (int, error) {
	return r.source.StaffUnreadCount(ctx, id)
}
func (r staffReadRecords) ListInbox(ctx context.Context, id int64, onlyUnread bool) ([]*StaffInboxThread, error) {
	rows, err := r.source.ListStaffInbox(ctx, id, onlyUnread)
	if rows == nil {
		return nil, err
	}
	result := make([]*StaffInboxThread, len(rows))
	for i, row := range rows {
		result[i] = (*StaffInboxThread)(values.StaffInboxFromRecord(row))
	}
	return result, err
}
func (r staffReadRecords) ListMessageableStaff(ctx context.Context, id int64) ([]*MessageableStaff, error) {
	rows, err := r.source.ListAddressableStaff(ctx, id)
	if rows == nil {
		return nil, err
	}
	result := make([]*MessageableStaff, len(rows))
	for i, row := range rows {
		result[i] = (*MessageableStaff)(values.MessageableStaffFromRecord(row))
	}
	return result, err
}
func (r staffReadRecords) IsMessageableStaff(ctx context.Context, id int64) (bool, error) {
	return r.source.IsAddressableStaff(ctx, id)
}
func (r staffReadRecords) StaffRoleKinds(ctx context.Context, ids []int64) (map[int64]string, error) {
	return r.source.StaffRoleKindsForAccounts(ctx, ids)
}
