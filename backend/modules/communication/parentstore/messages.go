package parentstore

import (
	"context"
	"time"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/parentpostgres"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"
	"github.com/uptrace/bun"
)

// NewParentMessageRepository returns the message log behind the existing
// repository contract.
func NewParentMessageRepository(db *bun.DB) usersModels.ParentMessageRepository {
	return &messageRepository{store: parentpostgres.NewMessageStore(postgresDatabase(db))}
}

type messageRepository struct{ store *parentpostgres.MessageStore }

func (r *messageRepository) Create(ctx context.Context, message *usersModels.ParentMessage) error {
	value := messageValue(message)
	if err := r.store.Create(ctx, value); err != nil {
		return err
	}
	// The thread preview is advanced off the row's own DB-stamped created_at, so
	// the identity and timestamps have to travel back to the caller.
	message.ID = value.ID
	message.CreatedAt = value.CreatedAt
	message.UpdatedAt = value.UpdatedAt
	message.SetTenantID(value.TenantID)
	return nil
}

func (r *messageRepository) Update(ctx context.Context, message *usersModels.ParentMessage) error {
	return r.store.Update(ctx, messageValue(message))
}

func (r *messageRepository) FindByID(ctx context.Context, id int64) (*usersModels.ParentMessage, error) {
	value, err := r.store.FindByID(ctx, id)
	return messageModel(value), err
}

func (r *messageRepository) FindByIDForUpdate(ctx context.Context, id int64) (*usersModels.ParentMessage, error) {
	value, err := r.store.FindByIDForUpdate(ctx, id)
	return messageModel(value), err
}

func (r *messageRepository) FindEventByRef(ctx context.Context, threadID int64, eventType, refTable string, refID int64) (*usersModels.ParentMessage, error) {
	value, err := r.store.FindEventByRef(ctx, threadID, eventType, refTable, refID)
	return messageModel(value), err
}

func (r *messageRepository) ListByThread(ctx context.Context, threadID int64, limit int) ([]*usersModels.ParentMessage, error) {
	values, err := r.store.ListByThread(ctx, threadID, limit)
	if err != nil {
		return nil, err
	}
	messages := make([]*usersModels.ParentMessage, 0, len(values))
	for _, value := range values {
		messages = append(messages, messageModel(value))
	}
	return messages, nil
}

func messageValue(message *usersModels.ParentMessage) *domain.ParentMessage {
	if message == nil {
		return nil
	}
	return &domain.ParentMessage{
		ID: message.ID, TenantID: message.GetTenantID(), ThreadID: message.ThreadID,
		StudentID: message.StudentID, SenderAccountID: message.SenderAccountID,
		SenderKind: message.SenderKind, SenderName: message.SenderName,
		StaffNameVisible: message.StaffNameVisible, Body: message.Body, Kind: message.Kind,
		EventType: message.EventType, EventActorKind: message.EventActorKind,
		RequestType: message.RequestType, RequestStatus: message.RequestStatus,
		Payload: message.Payload, RefTable: message.RefTable, RefID: message.RefID,
		AppliedAt: message.AppliedAt, AppliedBy: message.AppliedBy,
		DecisionReason: message.DecisionReason,
		CreatedAt:      message.CreatedAt, UpdatedAt: message.UpdatedAt,
	}
}

func messageModel(value *domain.ParentMessage) *usersModels.ParentMessage {
	if value == nil {
		return nil
	}
	message := &usersModels.ParentMessage{
		ThreadID: value.ThreadID, StudentID: value.StudentID,
		SenderAccountID: value.SenderAccountID, SenderKind: value.SenderKind,
		SenderName: value.SenderName, StaffNameVisible: value.StaffNameVisible,
		Body: value.Body, Kind: value.Kind, EventType: value.EventType,
		EventActorKind: value.EventActorKind, RequestType: value.RequestType,
		RequestStatus: value.RequestStatus, Payload: value.Payload,
		RefTable: value.RefTable, RefID: value.RefID, AppliedAt: value.AppliedAt,
		AppliedBy: value.AppliedBy, DecisionReason: value.DecisionReason,
	}
	message.ID = value.ID
	message.CreatedAt = value.CreatedAt
	message.UpdatedAt = value.UpdatedAt
	message.SetTenantID(value.TenantID)
	return message
}

// GuardianLister answers which guardians of a child hold a portal account and
// may therefore receive a message. People Directory owns those rows, so the
// composition root supplies this instead of Communication joining them.
type GuardianLister interface {
	ListGuardiansForStudent(ctx context.Context, studentID int64) ([]*usersModels.MessageableGuardian, error)
}

// NewParentMessageThreadRepository returns the thread store behind the existing
// repository contract. guardians supplies the recipient picker's rows.
func NewParentMessageThreadRepository(db *bun.DB, guardians GuardianLister) usersModels.ParentMessageThreadRepository {
	if guardians == nil {
		panic("communication parent store: guardian lister is required")
	}
	return &threadRepository{store: parentpostgres.NewThreadStore(postgresDatabase(db)), guardians: guardians}
}

type threadRepository struct {
	store     *parentpostgres.ThreadStore
	guardians GuardianLister
}

func (r *threadRepository) Create(ctx context.Context, thread *usersModels.ParentMessageThread) error {
	value := threadValue(thread)
	if err := r.store.Create(ctx, value); err != nil {
		return err
	}
	thread.ID = value.ID
	thread.CreatedAt = value.CreatedAt
	thread.UpdatedAt = value.UpdatedAt
	thread.SetTenantID(value.TenantID)
	return nil
}

func (r *threadRepository) Update(ctx context.Context, thread *usersModels.ParentMessageThread) error {
	return r.store.Update(ctx, threadValue(thread))
}

func (r *threadRepository) FindByID(ctx context.Context, id int64) (*usersModels.ParentMessageThread, error) {
	value, err := r.store.FindByID(ctx, id)
	return threadModel(value), err
}

func (r *threadRepository) FindByStudentGuardian(ctx context.Context, studentID, guardianAccountID int64) (*usersModels.ParentMessageThread, error) {
	value, err := r.store.FindByStudentGuardian(ctx, studentID, guardianAccountID)
	return threadModel(value), err
}

func (r *threadRepository) GetOrCreate(ctx context.Context, tenantID, studentID, guardianAccountID int64) (*usersModels.ParentMessageThread, error) {
	value, err := r.store.GetOrCreate(ctx, tenantID, studentID, guardianAccountID)
	if err != nil {
		return nil, err
	}
	return threadModel(value), nil
}

func (r *threadRepository) LockForMessageAppend(ctx context.Context, threadID int64) error {
	return r.store.LockForMessageAppend(ctx, threadID)
}

func (r *threadRepository) TouchLastMessage(ctx context.Context, threadID int64, at time.Time, messageID int64, senderKind, body string) error {
	return r.store.TouchLastMessage(ctx, threadID, at, messageID, senderKind, body)
}

func (r *threadRepository) ClaimStaffMessageNotification(ctx context.Context, threadID int64, cooldown time.Duration) (bool, error) {
	return r.store.ClaimStaffMessageNotification(ctx, threadID, cooldown)
}

func (r *threadRepository) ListGuardiansForStudent(ctx context.Context, studentID int64) ([]*usersModels.MessageableGuardian, error) {
	return r.guardians.ListGuardiansForStudent(ctx, studentID)
}

func threadValue(thread *usersModels.ParentMessageThread) *domain.ParentMessageThread {
	if thread == nil {
		return nil
	}
	return &domain.ParentMessageThread{
		ID: thread.ID, TenantID: thread.GetTenantID(), StudentID: thread.StudentID,
		GuardianAccountID: thread.GuardianAccountID, LastMessageAt: thread.LastMessageAt,
		LastMessageID: thread.LastMessageID, LastSenderKind: thread.LastSenderKind,
		LastMessageBody: thread.LastMessageBody, StaffHandledUpToAt: thread.StaffHandledUpToAt,
		StaffHandledUpToMessageID:      thread.StaffHandledUpToMessageID,
		LastStaffMessageNotificationAt: thread.LastStaffMessageNotificationAt,
		CreatedAt:                      thread.CreatedAt, UpdatedAt: thread.UpdatedAt,
	}
}

func threadModel(value *domain.ParentMessageThread) *usersModels.ParentMessageThread {
	if value == nil {
		return nil
	}
	thread := &usersModels.ParentMessageThread{
		StudentID: value.StudentID, GuardianAccountID: value.GuardianAccountID,
		LastMessageAt: value.LastMessageAt, LastMessageID: value.LastMessageID,
		LastSenderKind: value.LastSenderKind, LastMessageBody: value.LastMessageBody,
		StaffHandledUpToAt:             value.StaffHandledUpToAt,
		StaffHandledUpToMessageID:      value.StaffHandledUpToMessageID,
		LastStaffMessageNotificationAt: value.LastStaffMessageNotificationAt,
	}
	thread.ID = value.ID
	thread.CreatedAt = value.CreatedAt
	thread.UpdatedAt = value.UpdatedAt
	thread.SetTenantID(value.TenantID)
	return thread
}
