package parentpostgres

import (
	"context"
	"time"

	recordports "github.com/moto-nrw/project-phoenix/modules/communication/ports"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"
	"github.com/uptrace/bun"
)

// NewParentMessageRepository returns the message log behind the existing
// repository contract.
func NewParentMessageRepository(db *bun.DB) recordports.ParentMessageRepository {
	return &messageRepository{store: NewMessageStore(NewParentRecordDatabase(db))}
}

type messageRepository struct{ store *MessageStore }

func (r *messageRepository) Create(ctx context.Context, message *communication.PortalParentMessage) error {
	value, err := messageValue(message)
	if err != nil {
		return err
	}
	// Preserve the native source's existing JSON normalization on success and failure.
	if message != nil {
		message.Payload, err = domain.ParentPayloadFromFacts(value.Payload)
		if err != nil {
			return err
		}
	}
	if err := r.store.Create(ctx, value); err != nil {
		return err
	}
	// The thread preview is advanced off the row's own DB-stamped created_at, so
	// the identity and timestamps have to travel back to the caller.
	message.ID = value.ID
	message.CreatedAt = value.CreatedAt
	message.UpdatedAt = value.UpdatedAt
	message.TenantID = value.TenantID
	return nil
}

func (r *messageRepository) Update(ctx context.Context, message *communication.PortalParentMessage) error {
	value, err := messageValue(message)
	if err != nil {
		return err
	}
	return r.store.Update(ctx, value)
}

func (r *messageRepository) FindByID(ctx context.Context, id int64) (*communication.PortalParentMessage, error) {
	value, err := r.store.FindByID(ctx, id)
	mapped, mapErr := messageModel(value)
	if err != nil {
		return mapped, err
	}
	return mapped, mapErr
}

func (r *messageRepository) FindByIDForUpdate(ctx context.Context, id int64) (*communication.PortalParentMessage, error) {
	value, err := r.store.FindByIDForUpdate(ctx, id)
	mapped, mapErr := messageModel(value)
	if err != nil {
		return mapped, err
	}
	return mapped, mapErr
}

func (r *messageRepository) FindEventByRef(ctx context.Context, threadID int64, eventType, refTable string, refID int64) (*communication.PortalParentMessage, error) {
	value, err := r.store.FindEventByRef(ctx, threadID, eventType, refTable, refID)
	mapped, mapErr := messageModel(value)
	if err != nil {
		return mapped, err
	}
	return mapped, mapErr
}

func (r *messageRepository) ListByThread(ctx context.Context, threadID int64, limit int) ([]*communication.PortalParentMessage, error) {
	values, err := r.store.ListByThread(ctx, threadID, limit)
	if err != nil {
		return nil, err
	}
	messages := make([]*communication.PortalParentMessage, 0, len(values))
	for _, value := range values {
		mapped, err := messageModel(value)
		if err != nil {
			return nil, err
		}
		messages = append(messages, mapped)
	}
	return messages, nil
}

func messageValue(message *communication.PortalParentMessage) (*domain.ParentMessage, error) {
	if message == nil {
		return nil, nil
	}
	payload, err := domain.ParentPayloadToFacts(message.Payload)
	if err != nil {
		return nil, err
	}
	return &domain.ParentMessage{
		ID: message.ID, TenantID: message.TenantID, ThreadID: message.ThreadID,
		StudentID: message.StudentID, SenderAccountID: message.SenderAccountID,
		SenderKind: message.SenderKind, SenderName: message.SenderName,
		StaffNameVisible: message.StaffNameVisible, Body: message.Body, Kind: message.Kind,
		EventType: message.EventType, EventActorKind: message.EventActorKind,
		RequestType: message.RequestType, RequestStatus: message.RequestStatus,
		Payload: payload, RefTable: message.RefTable, RefID: message.RefID,
		AppliedAt: message.AppliedAt, AppliedBy: message.AppliedBy,
		DecisionReason: message.DecisionReason, GroupThreadID: message.GroupThreadID,
		CreatedAt: message.CreatedAt, UpdatedAt: message.UpdatedAt,
	}, nil
}

func messageModel(value *domain.ParentMessage) (*communication.PortalParentMessage, error) {
	if value == nil {
		return nil, nil
	}
	payload, err := domain.ParentPayloadFromFacts(value.Payload)
	if err != nil {
		return nil, err
	}
	message := &communication.PortalParentMessage{
		ThreadID: value.ThreadID, StudentID: value.StudentID,
		SenderAccountID: value.SenderAccountID, SenderKind: value.SenderKind,
		SenderName: value.SenderName, StaffNameVisible: value.StaffNameVisible,
		Body: value.Body, Kind: value.Kind, EventType: value.EventType,
		EventActorKind: value.EventActorKind, RequestType: value.RequestType,
		RequestStatus: value.RequestStatus, Payload: payload,
		RefTable: value.RefTable, RefID: value.RefID, AppliedAt: value.AppliedAt,
		AppliedBy: value.AppliedBy, DecisionReason: value.DecisionReason, GroupThreadID: value.GroupThreadID,
	}
	message.ID = value.ID
	message.CreatedAt = value.CreatedAt
	message.UpdatedAt = value.UpdatedAt
	message.TenantID = value.TenantID
	return message, nil
}

// NewParentMessageThreadRepository returns the thread store behind the existing
// repository contract. guardians supplies the recipient picker's rows,
// reachable the children that have such a recipient.
func NewParentMessageThreadRepository(db *bun.DB, guardians recordports.GuardianLister, reachable recordports.ReachableStudentLister) recordports.ParentMessageThreadRepository {
	if guardians == nil {
		panic("communication parent store: guardian lister is required")
	}
	if reachable == nil {
		panic("communication parent store: reachable student lister is required")
	}
	return &threadRepository{store: NewThreadStore(NewParentRecordDatabase(db)), guardians: guardians, reachable: reachable}
}

type threadRepository struct {
	store     *ThreadStore
	guardians recordports.GuardianLister
	reachable recordports.ReachableStudentLister
}

func (r *threadRepository) Create(ctx context.Context, thread *communication.PortalParentMessageThread) error {
	value := threadValue(thread)
	if err := r.store.Create(ctx, value); err != nil {
		return err
	}
	thread.ID = value.ID
	thread.CreatedAt = value.CreatedAt
	thread.UpdatedAt = value.UpdatedAt
	thread.TenantID = value.TenantID
	return nil
}

func (r *threadRepository) Update(ctx context.Context, thread *communication.PortalParentMessageThread) error {
	return r.store.Update(ctx, threadValue(thread))
}

func (r *threadRepository) FindByID(ctx context.Context, id int64) (*communication.PortalParentMessageThread, error) {
	value, err := r.store.FindByID(ctx, id)
	return threadModel(value), err
}

func (r *threadRepository) FindByStudentGuardian(ctx context.Context, studentID, guardianAccountID int64) (*communication.PortalParentMessageThread, error) {
	value, err := r.store.FindByStudentGuardian(ctx, studentID, guardianAccountID)
	return threadModel(value), err
}

func (r *threadRepository) GetOrCreate(ctx context.Context, tenantID, studentID, guardianAccountID int64) (*communication.PortalParentMessageThread, error) {
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

func (r *threadRepository) ListGuardiansForStudent(ctx context.Context, studentID int64) ([]*communication.ParentGuardianRecipient, error) {
	return r.guardians(ctx, studentID)
}

func (r *threadRepository) ListStudentIDsWithMessageableGuardian(ctx context.Context) ([]int64, error) {
	return r.reachable(ctx)
}

func threadValue(thread *communication.PortalParentMessageThread) *domain.ParentMessageThread {
	if thread == nil {
		return nil
	}
	return &domain.ParentMessageThread{
		ID: thread.ID, TenantID: thread.TenantID, StudentID: thread.StudentID,
		GuardianAccountID: thread.GuardianAccountID, LastMessageAt: thread.LastMessageAt,
		LastMessageID: thread.LastMessageID, LastSenderKind: thread.LastSenderKind,
		LastMessageBody: thread.LastMessageBody, StaffHandledUpToAt: thread.StaffHandledUpToAt,
		StaffHandledUpToMessageID:      thread.StaffHandledUpToMessageID,
		StaffMarkedUnreadAt:            thread.StaffMarkedUnreadAt,
		StaffMarkedUnreadByAccountID:   thread.StaffMarkedUnreadByAccountID,
		LastStaffMessageNotificationAt: thread.LastStaffMessageNotificationAt,
		CreatedAt:                      thread.CreatedAt, UpdatedAt: thread.UpdatedAt,
	}
}

func threadModel(value *domain.ParentMessageThread) *communication.PortalParentMessageThread {
	if value == nil {
		return nil
	}
	thread := &communication.PortalParentMessageThread{
		StudentID: value.StudentID, GuardianAccountID: value.GuardianAccountID,
		LastMessageAt: value.LastMessageAt, LastMessageID: value.LastMessageID,
		LastSenderKind: value.LastSenderKind, LastMessageBody: value.LastMessageBody,
		StaffHandledUpToAt:             value.StaffHandledUpToAt,
		StaffHandledUpToMessageID:      value.StaffHandledUpToMessageID,
		StaffMarkedUnreadAt:            value.StaffMarkedUnreadAt,
		StaffMarkedUnreadByAccountID:   value.StaffMarkedUnreadByAccountID,
		LastStaffMessageNotificationAt: value.LastStaffMessageNotificationAt,
	}
	thread.ID = value.ID
	thread.CreatedAt = value.CreatedAt
	thread.UpdatedAt = value.UpdatedAt
	thread.TenantID = value.TenantID
	return thread
}
