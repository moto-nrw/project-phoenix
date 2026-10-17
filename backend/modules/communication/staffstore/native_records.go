package staffstore

import (
	"context"
	"time"

	users "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/communication"
)

// Existing bound stores advertise native public facets beside retained callers.
// No second store or lookup is constructed.
type StaffMessageThreadRepository interface {
	users.StaffMessageThreadRepository
	communication.StaffThreadRecords
}
type StaffMessageRepository interface {
	users.StaffMessageRepository
	communication.StaffMessageRecords
}
type StaffMessageReadRepository interface {
	users.StaffMessageReadRepository
	communication.StaffReadRecords
}

func (r *threadRepository) GetOrCreateStaffThread(ctx context.Context, a, b int64) (*communication.StaffThreadRecord, error) {
	row, err := r.GetOrCreateDirect(ctx, a, b)
	return staffThreadRecord(row), err
}
func (r *threadRepository) FindStaffThread(ctx context.Context, id int64) (*communication.StaffThreadRecord, error) {
	row, err := r.FindByID(ctx, id)
	return staffThreadRecord(row), err
}
func (r *threadRepository) LockStaffThreadForAppend(ctx context.Context, id int64) error {
	return r.LockForMessageAppend(ctx, id)
}
func (r *threadRepository) AdvanceStaffThreadPreview(ctx context.Context, id int64, at time.Time, messageID, senderID int64, body string) error {
	return r.TouchLastMessage(ctx, id, at, messageID, senderID, body)
}
func (r *threadRepository) StaffParticipantAccountIDs(ctx context.Context, id int64) ([]int64, error) {
	return r.ParticipantAccountIDs(ctx, id)
}
func (r *threadRepository) HasStaffParticipant(ctx context.Context, id, accountID int64) (bool, error) {
	return r.IsParticipant(ctx, id, accountID)
}
func (r *threadRepository) DeleteEmptyStaffThreads(ctx context.Context, before time.Time) (int64, error) {
	return r.DeleteEmpty(ctx, before)
}
func staffThreadRecord(row *users.StaffMessageThread) *communication.StaffThreadRecord {
	if row == nil {
		return nil
	}
	return &communication.StaffThreadRecord{
		ID: row.ID, TenantID: row.GetTenantID(), ParticipantKey: row.ParticipantKey, Kind: row.Kind,
		LastMessageAt: row.LastMessageAt, LastMessageID: row.LastMessageID, LastMessageBody: row.LastMessageBody, LastSenderAccountID: row.LastSenderAccountID,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func (r *messageRepository) CreateStaffMessage(ctx context.Context, input *communication.StaffMessageRecord) error {
	var row *users.StaffMessage
	if input != nil {
		row = &users.StaffMessage{ThreadID: input.ThreadID, SenderAccountID: input.SenderAccountID, SenderName: input.SenderName, Body: input.Body}
		row.ID = input.ID
		row.CreatedAt = input.CreatedAt
		row.UpdatedAt = input.UpdatedAt
		row.SetTenantID(input.TenantID)
	}
	err := r.Create(ctx, row)
	if input != nil {
		*input = *staffMessageRecord(row)
	}
	return err
}
func (r *messageRepository) ListStaffMessages(ctx context.Context, id int64) ([]*communication.StaffMessageRecord, error) {
	rows, err := r.ListByThread(ctx, id)
	return staffRecordSlice(rows, staffMessageRecord), err
}
func (r *messageRepository) DeleteStaffMessagesBefore(ctx context.Context, before time.Time) (int64, error) {
	return r.DeleteOlderThan(ctx, before)
}
func staffMessageRecord(row *users.StaffMessage) *communication.StaffMessageRecord {
	if row == nil {
		return nil
	}
	return &communication.StaffMessageRecord{ID: row.ID, TenantID: row.GetTenantID(), ThreadID: row.ThreadID, SenderAccountID: row.SenderAccountID, SenderName: row.SenderName, Body: row.Body, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

func (r *readRepository) MarkStaffReadUpTo(ctx context.Context, id, accountID int64, at time.Time, messageID int64) error {
	return r.MarkReadUpTo(ctx, id, accountID, at, messageID)
}
func (r *readRepository) StaffUnreadCount(ctx context.Context, accountID int64) (int, error) {
	return r.UnreadCount(ctx, accountID)
}
func (r *readRepository) ListStaffInbox(ctx context.Context, accountID int64, unread bool) ([]*communication.StaffInboxRecord, error) {
	rows, err := r.ListInbox(ctx, accountID, unread)
	return staffRecordSlice(rows, func(row *users.StaffInboxThread) *communication.StaffInboxRecord {
		if row == nil {
			return nil
		}
		return &communication.StaffInboxRecord{ThreadID: row.ThreadID, TenantID: row.TenantID, CounterpartAccountID: row.CounterpartAccountID, CounterpartName: row.CounterpartName, CounterpartRoleKind: row.CounterpartRoleKind, LastMessageAt: row.LastMessageAt, LastMessageBody: row.LastMessageBody, LastSenderAccountID: row.LastSenderAccountID, UnreadCount: row.UnreadCount}
	}), err
}
func (r *readRepository) ListAddressableStaff(ctx context.Context, accountID int64) ([]*communication.AddressableStaffRecord, error) {
	rows, err := r.ListMessageableStaff(ctx, accountID)
	return staffRecordSlice(rows, func(row *users.MessageableStaff) *communication.AddressableStaffRecord {
		if row == nil {
			return nil
		}
		return &communication.AddressableStaffRecord{AccountID: row.AccountID, Name: row.Name, RoleKind: row.RoleKind}
	}), err
}
func (r *readRepository) IsAddressableStaff(ctx context.Context, accountID int64) (bool, error) {
	return r.IsMessageableStaff(ctx, accountID)
}
func (r *readRepository) StaffRoleKindsForAccounts(ctx context.Context, ids []int64) (map[int64]string, error) {
	return r.StaffRoleKinds(ctx, ids)
}
func staffRecordSlice[A, B any](rows []*A, convert func(*A) *B) []*B {
	if rows == nil {
		return nil
	}
	result := make([]*B, len(rows))
	for i, row := range rows {
		result[i] = convert(row)
	}
	return result
}
