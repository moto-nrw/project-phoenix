package staffstore

import (
	"context"
	"time"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/staffinbox"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/staffpostgres"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"
	"github.com/uptrace/bun"
)

// NewStaffMessageThreadRepository returns the conversation store behind the
// existing repository contract.
func NewStaffMessageThreadRepository(db *bun.DB) usersModels.StaffMessageThreadRepository {
	return &threadRepository{store: staffpostgres.NewThreadStore(postgresDatabase(db))}
}

type threadRepository struct{ store *staffpostgres.ThreadStore }

func (r *threadRepository) GetOrCreateDirect(ctx context.Context, accountA, accountB int64) (*usersModels.StaffMessageThread, error) {
	value, err := r.store.GetOrCreate(ctx,
		usersModels.DirectParticipantKey(accountA, accountB),
		usersModels.StaffMessageThreadKindDirect,
		accountA, accountB)
	if err != nil {
		return nil, err
	}
	return threadModel(value), nil
}

func (r *threadRepository) FindByID(ctx context.Context, id int64) (*usersModels.StaffMessageThread, error) {
	value, err := r.store.FindByID(ctx, id)
	if err != nil || value == nil {
		return nil, err
	}
	return threadModel(value), nil
}

func (r *threadRepository) LockForMessageAppend(ctx context.Context, threadID int64) error {
	return r.store.LockForMessageAppend(ctx, threadID)
}

func (r *threadRepository) TouchLastMessage(ctx context.Context, threadID int64, at time.Time, messageID, senderAccountID int64, body string) error {
	return r.store.TouchLastMessage(ctx, threadID, at, messageID, senderAccountID, body)
}

func (r *threadRepository) ParticipantAccountIDs(ctx context.Context, threadID int64) ([]int64, error) {
	return r.store.ParticipantAccountIDs(ctx, threadID)
}

func (r *threadRepository) IsParticipant(ctx context.Context, threadID, accountID int64) (bool, error) {
	return r.store.IsParticipant(ctx, threadID, accountID)
}

func (r *threadRepository) DeleteEmpty(ctx context.Context, createdBefore time.Time) (int64, error) {
	return r.store.DeleteEmpty(ctx, createdBefore)
}

func threadModel(value *domain.StaffMessageThread) *usersModels.StaffMessageThread {
	thread := &usersModels.StaffMessageThread{
		ParticipantKey: value.ParticipantKey, Kind: value.Kind,
		LastMessageAt: value.LastMessageAt, LastMessageID: value.LastMessageID,
		LastMessageBody: value.LastMessageBody, LastSenderAccountID: value.LastSenderAccountID,
	}
	thread.ID = value.ID
	thread.CreatedAt = value.CreatedAt
	thread.UpdatedAt = value.UpdatedAt
	thread.SetTenantID(value.TenantID)
	return thread
}

// NewStaffMessageRepository returns the message log behind the existing
// repository contract.
func NewStaffMessageRepository(db *bun.DB) usersModels.StaffMessageRepository {
	return &messageRepository{store: staffpostgres.NewMessageStore(postgresDatabase(db))}
}

type messageRepository struct{ store *staffpostgres.MessageStore }

func (r *messageRepository) Create(ctx context.Context, message *usersModels.StaffMessage) error {
	value := &domain.StaffMessage{
		ID: message.ID, TenantID: message.GetTenantID(), ThreadID: message.ThreadID,
		SenderAccountID: message.SenderAccountID, SenderName: message.SenderName, Body: message.Body,
		CreatedAt: message.CreatedAt, UpdatedAt: message.UpdatedAt,
	}
	if err := r.store.Create(ctx, value); err != nil {
		return err
	}
	// The read cursor and the thread preview are advanced off the row's own
	// DB-stamped created_at, so the identity and timestamps travel back.
	message.ID = value.ID
	message.CreatedAt = value.CreatedAt
	message.UpdatedAt = value.UpdatedAt
	message.SetTenantID(value.TenantID)
	return nil
}

func (r *messageRepository) ListByThread(ctx context.Context, threadID int64) ([]*usersModels.StaffMessage, error) {
	values, err := r.store.ListByThread(ctx, threadID)
	if err != nil {
		return nil, err
	}
	messages := make([]*usersModels.StaffMessage, 0, len(values))
	for _, value := range values {
		message := &usersModels.StaffMessage{
			ThreadID: value.ThreadID, SenderAccountID: value.SenderAccountID,
			SenderName: value.SenderName, Body: value.Body,
		}
		message.ID = value.ID
		message.CreatedAt = value.CreatedAt
		message.UpdatedAt = value.UpdatedAt
		message.SetTenantID(value.TenantID)
		messages = append(messages, message)
	}
	return messages, nil
}

func (r *messageRepository) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	return r.store.DeleteOlderThan(ctx, cutoff)
}

// ColleagueDirectory answers who counts as a colleague at the current school
// and which side of the school they sit on. People Directory owns the person
// rows behind it, and School Membership and Identity & Access own the facts it
// filters through, so the composition root supplies it instead of
// Communication joining them.
//
// ListMessageableStaff and IsMessageableStaff MUST apply the same colleague
// relation: the picker decides what a user is offered, the predicate what the
// API accepts, and any drift between them is an authorization hole.
type ColleagueDirectory interface {
	ListMessageableStaff(ctx context.Context, viewerAccountID int64) ([]*usersModels.MessageableStaff, error)
	IsMessageableStaff(ctx context.Context, accountID int64) (bool, error)
	StaffRoleKinds(ctx context.Context, accountIDs []int64) (map[int64]string, error)
}

// NewStaffMessageReadRepository returns the read cursors, the inbox and badge
// projection and the colleague lookups behind the existing repository
// contract. colleagues is required; a graph without it must not start.
func NewStaffMessageReadRepository(db *bun.DB, colleagues ColleagueDirectory) usersModels.StaffMessageReadRepository {
	if colleagues == nil {
		panic("communication staff store: colleague directory is required")
	}
	return &readRepository{
		ColleagueDirectory: colleagues,
		cursors:            staffpostgres.NewReadCursorStore(postgresDatabase(db)),
		inbox:              staffinbox.New(inboxDatabase(db)),
	}
}

type readRepository struct {
	ColleagueDirectory
	cursors *staffpostgres.ReadCursorStore
	inbox   *staffinbox.Projection
}

func (r *readRepository) MarkReadUpTo(ctx context.Context, threadID, accountID int64, at time.Time, messageID int64) error {
	return r.cursors.MarkReadUpTo(ctx, threadID, accountID, at, messageID)
}

func (r *readRepository) UnreadCount(ctx context.Context, accountID int64) (int, error) {
	return r.inbox.UnreadCount(ctx, accountID)
}

func (r *readRepository) ListInbox(ctx context.Context, accountID int64, onlyUnread bool) ([]*usersModels.StaffInboxThread, error) {
	values, err := r.inbox.ListInbox(ctx, accountID, onlyUnread)
	if err != nil {
		return nil, err
	}
	rows := make([]*usersModels.StaffInboxThread, 0, len(values))
	for _, value := range values {
		rows = append(rows, &usersModels.StaffInboxThread{
			ThreadID: value.ThreadID, TenantID: value.TenantID,
			CounterpartAccountID: value.CounterpartAccountID, CounterpartName: value.CounterpartName,
			LastMessageAt: value.LastMessageAt, LastMessageBody: value.LastMessageBody,
			LastSenderAccountID: value.LastSenderAccountID, UnreadCount: value.UnreadCount,
		})
	}
	return rows, nil
}
