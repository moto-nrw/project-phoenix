package staffpostgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"
	"github.com/uptrace/bun"
)

const (
	staffMessagesTable              = "users.staff_messages"
	tableExprStaffMessagesAsMessage = staffMessagesTable + ` AS "staff_message"`
)

// MessageStore owns the append-only staff message log.
type MessageStore struct{ store }

// NewMessageStore builds the message store over the ambient transaction
// runtime.
func NewMessageStore(database Database) *MessageStore {
	return &MessageStore{store: newStore(database)}
}

type staffMessageRow struct {
	bun.BaseModel   `bun:"table:users.staff_messages,alias:staff_message"`
	ID              int64     `bun:"id,pk,autoincrement"`
	CreatedAt       time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt       time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID        int64     `bun:"tenant_id,notnull"`
	ThreadID        int64     `bun:"thread_id,notnull"`
	SenderAccountID int64     `bun:"sender_account_id,notnull"`
	SenderName      string    `bun:"sender_name,notnull"`
	Body            string    `bun:"body,notnull"`
}

func (r *staffMessageRow) value() *domain.StaffMessage {
	return &domain.StaffMessage{
		ID: r.ID, TenantID: r.TenantID, ThreadID: r.ThreadID,
		SenderAccountID: r.SenderAccountID, SenderName: r.SenderName, Body: r.Body,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// Create appends one message and reads the persisted identity back.
//
// created_at is deliberately NOT set by the application: the column defaults to
// clock_timestamp() so the message is stamped at insert time rather than at
// transaction start. The RETURNING clause pulls that value (and the id) back,
// because both halves of the (created_at, id) composite are what the caller
// feeds into TouchLastMessage and the read cursor — computing them in Go would
// reintroduce exactly the skew the database default exists to avoid.
func (s *MessageStore) Create(ctx context.Context, message *domain.StaffMessage) error {
	if message == nil {
		return errors.New("create staff message: message is required")
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	if message.TenantID == 0 {
		message.TenantID = tenantID
	}
	row := &staffMessageRow{
		ID: message.ID, TenantID: message.TenantID, ThreadID: message.ThreadID,
		SenderAccountID: message.SenderAccountID, SenderName: message.SenderName, Body: message.Body,
		CreatedAt: message.CreatedAt, UpdatedAt: message.UpdatedAt,
	}
	if _, err := db.NewInsert().
		Model(row).
		ModelTableExpr(staffMessagesTable).
		ExcludeColumn("created_at", "updated_at").
		Returning("id, created_at, updated_at").
		Exec(ctx); err != nil {
		return fmt.Errorf("create staff message: %w", err)
	}
	message.ID = row.ID
	message.CreatedAt = row.CreatedAt
	message.UpdatedAt = row.UpdatedAt
	return nil
}

// ListByThread returns the thread's messages oldest-first, which is the order
// the chat window renders. Ties on created_at break by id, matching the
// composite the unread cursor compares against.
func (s *MessageStore) ListByThread(ctx context.Context, threadID int64) ([]*domain.StaffMessage, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []staffMessageRow
	query := db.NewSelect().
		Model(&rows).
		ModelTableExpr(tableExprStaffMessagesAsMessage).
		Where(`"staff_message".thread_id = ?`, threadID).
		OrderExpr(`"staff_message".created_at ASC, "staff_message".id ASC`)
	query = withTenant(query, "staff_message", tenantID)

	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("list staff messages by thread: %w", err)
	}
	messages := make([]*domain.StaffMessage, 0, len(rows))
	for i := range rows {
		messages = append(messages, rows[i].value())
	}
	return messages, nil
}

// DeleteOlderThan removes messages created before the cutoff across the whole
// tenant (retention housekeeping) and reports how many rows went.
func (s *MessageStore) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, err
	}
	query := db.NewDelete().
		Model((*staffMessageRow)(nil)).
		ModelTableExpr(tableExprStaffMessagesAsMessage).
		Where(`"staff_message".created_at < ?`, cutoff)
	query = withTenant(query, "staff_message", tenantID)

	res, err := query.Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("delete old staff messages: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("delete old staff messages: %w", err)
	}
	return affected, nil
}
