package staffstore

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/staffpostgres"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"
	"github.com/uptrace/bun"
)

// NewStaffMessageThreadRepository binds the native contract to the existing store.
func NewStaffMessageThreadRepository(db *bun.DB) communication.StaffThreadRecords {
	return &threadRepository{store: staffpostgres.NewThreadStore(postgresDatabase(db))}
}

type threadRepository struct{ store *staffpostgres.ThreadStore }

func (r *threadRepository) GetOrCreateStaffThread(ctx context.Context, a, b int64) (*communication.StaffThreadRecord, error) {
	row, err := r.store.GetOrCreate(ctx, domain.DirectParticipantKey(a, b), domain.StaffMessageThreadKindDirect, a, b)
	if err != nil {
		return nil, err
	}
	return (*communication.StaffThreadRecord)(row), nil
}
func (r *threadRepository) FindStaffThread(ctx context.Context, id int64) (*communication.StaffThreadRecord, error) {
	row, err := r.store.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return (*communication.StaffThreadRecord)(row), nil
}
func (r *threadRepository) LockStaffThreadForAppend(ctx context.Context, id int64) error {
	return r.store.LockForMessageAppend(ctx, id)
}
func (r *threadRepository) AdvanceStaffThreadPreview(ctx context.Context, id int64, at time.Time, messageID, senderID int64, body string) error {
	return r.store.TouchLastMessage(ctx, id, at, messageID, senderID, body)
}
func (r *threadRepository) StaffParticipantAccountIDs(ctx context.Context, id int64) ([]int64, error) {
	return r.store.ParticipantAccountIDs(ctx, id)
}
func (r *threadRepository) HasStaffParticipant(ctx context.Context, id, accountID int64) (bool, error) {
	return r.store.IsParticipant(ctx, id, accountID)
}
func (r *threadRepository) DeleteEmptyStaffThreads(ctx context.Context, before time.Time) (int64, error) {
	return r.store.DeleteEmpty(ctx, before)
}

// NewStaffMessageRepository binds the native log to the existing SQL store.
func NewStaffMessageRepository(db *bun.DB) communication.StaffMessageRecords {
	return &messageRepository{store: staffpostgres.NewMessageStore(postgresDatabase(db))}
}

type messageRepository struct{ store *staffpostgres.MessageStore }

func (r *messageRepository) CreateStaffMessage(ctx context.Context, input *communication.StaffMessageRecord) error {
	// Keep the original success-only copyback. The store may assign the tenant
	// before an insert fails; that partial value was never exposed by this seam.
	row := domain.StaffMessage(*input)
	if err := r.store.Create(ctx, &row); err != nil {
		return err
	}
	input.ID = row.ID
	input.TenantID = row.TenantID
	input.CreatedAt = row.CreatedAt
	input.UpdatedAt = row.UpdatedAt
	return nil
}
func (r *messageRepository) ListStaffMessages(ctx context.Context, id int64) ([]*communication.StaffMessageRecord, error) {
	rows, err := r.store.ListByThread(ctx, id)
	if err != nil {
		return nil, err
	}
	result := make([]*communication.StaffMessageRecord, 0, len(rows))
	for _, row := range rows {
		result = append(result, (*communication.StaffMessageRecord)(row))
	}
	return result, nil
}
func (r *messageRepository) DeleteStaffMessagesBefore(ctx context.Context, before time.Time) (int64, error) {
	return r.store.DeleteOlderThan(ctx, before)
}

// NewStaffMessageReadRepository binds the existing cursors and inbox to the
// exact colleague relation supplied by People Directory at the root.
func NewStaffMessageReadRepository(db *bun.DB, colleagues communication.StaffColleagueReads, inbox StaffInbox) communication.StaffReadRecords {
	if colleagues == nil {
		panic("communication staff store: colleague directory is required")
	}
	return &readRepository{StaffColleagueReads: colleagues, cursors: staffpostgres.NewReadCursorStore(postgresDatabase(db)), inbox: inbox}
}

type readRepository struct {
	communication.StaffColleagueReads
	cursors *staffpostgres.ReadCursorStore
	inbox   StaffInbox
}

func (r *readRepository) MarkStaffReadUpTo(ctx context.Context, id, accountID int64, at time.Time, messageID int64) error {
	return r.cursors.MarkReadUpTo(ctx, id, accountID, at, messageID)
}
func (r *readRepository) StaffUnreadCount(ctx context.Context, accountID int64) (int, error) {
	return r.inbox.UnreadCount(ctx, accountID)
}
func (r *readRepository) ListStaffInbox(ctx context.Context, accountID int64, unread bool) ([]*communication.StaffInboxRecord, error) {
	values, err := r.inbox.ListInbox(ctx, accountID, unread)
	if err != nil {
		return nil, err
	}
	rows := make([]*communication.StaffInboxRecord, 0, len(values))
	for _, value := range values {
		rows = append(rows, &communication.StaffInboxRecord{ThreadID: value.ThreadID, TenantID: value.TenantID, CounterpartAccountID: value.CounterpartAccountID, CounterpartName: value.CounterpartName, LastMessageAt: value.LastMessageAt, LastMessageBody: value.LastMessageBody, LastSenderAccountID: value.LastSenderAccountID, UnreadCount: value.UnreadCount})
	}
	return rows, nil
}
