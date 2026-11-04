package staffpostgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
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
	ClientMessageID string `bun:"client_message_id,nullzero"`
	SendReplayed    bool   `bun:"send_replayed,scanonly"`
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

func (r *staffMessageRow) value() *communication.StaffMessageRecord {
	return &communication.StaffMessageRecord{
		ID: r.ID, TenantID: r.TenantID, ThreadID: r.ThreadID,
		SenderAccountID: r.SenderAccountID, SenderName: r.SenderName, ClientMessageID: r.ClientMessageID, SendReplayed: r.SendReplayed, Body: r.Body,
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
func (s *MessageStore) CreateStaffMessage(ctx context.Context, input *communication.StaffMessageRecord) error {
	// Preserve the facade's success-only copyback, including its nil-input contract.
	message := *input
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	if message.TenantID == 0 {
		message.TenantID = tenantID
	}
	row := &staffMessageRow{
		ID: message.ID, TenantID: message.TenantID, ThreadID: message.ThreadID,
		SenderAccountID: message.SenderAccountID, SenderName: message.SenderName, ClientMessageID: message.ClientMessageID, SendReplayed: message.SendReplayed, Body: message.Body,
		CreatedAt: message.CreatedAt, UpdatedAt: message.UpdatedAt,
	}
	result, err := db.NewInsert().
		Model(row).
		ModelTableExpr(staffMessagesTable).
		ExcludeColumn("created_at", "updated_at").
		On("CONFLICT (tenant_id, sender_account_id, client_message_id) WHERE client_message_id IS NOT NULL DO UPDATE").
		Set("client_message_id = EXCLUDED.client_message_id").
		Where("staff_messages.thread_id = EXCLUDED.thread_id AND staff_messages.body = EXCLUDED.body").
		Returning("staff_messages.*, (xmax <> 0) AS send_replayed").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("create staff message: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return communication.ErrMessageSendConflict
	}
	*input = *row.value()
	return nil
}

// ListByThread returns the thread's messages oldest-first, which is the order
// the chat window renders. Ties on created_at break by id, matching the
// composite the unread cursor compares against.
func (s *MessageStore) ListStaffMessages(ctx context.Context, threadID int64) ([]*communication.StaffMessageRecord, error) {
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
	messages := make([]*communication.StaffMessageRecord, 0, len(rows))
	for i := range rows {
		messages = append(messages, rows[i].value())
	}
	return messages, nil
}

// DeleteOlderThan removes messages created before the cutoff across the whole
// tenant (retention housekeeping) and reports how many rows went.
func (s *MessageStore) DeleteStaffMessagesBefore(ctx context.Context, cutoff time.Time) (int64, error) {
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
	if affected > 0 {
		if _, err := db.ExecContext(ctx, `
			WITH previews AS (
				SELECT thread.id, latest.id AS last_message_id, latest.created_at AS last_message_at,
					latest.body AS last_message_body, latest.sender_account_id AS last_sender_account_id
				FROM users.staff_message_threads AS thread
				LEFT JOIN LATERAL (
					SELECT id, created_at, body, sender_account_id
					FROM users.staff_messages
					WHERE thread_id = thread.id AND tenant_id = thread.tenant_id
					ORDER BY created_at DESC, id DESC
					LIMIT 1
				) AS latest ON TRUE
				WHERE thread.tenant_id = ?
					AND (thread.last_message_at < ? OR NOT EXISTS (
						SELECT 1 FROM users.staff_messages current
						WHERE current.id = thread.last_message_id AND current.tenant_id = thread.tenant_id
					))
			)
			UPDATE users.staff_message_threads AS thread
			SET last_message_id = previews.last_message_id,
				last_message_at = previews.last_message_at,
				last_message_body = COALESCE(previews.last_message_body, ''),
				last_sender_account_id = previews.last_sender_account_id
			FROM previews
			WHERE thread.id = previews.id AND thread.tenant_id = ?
		`, tenantID, cutoff, tenantID); err != nil {
			return 0, fmt.Errorf("refresh staff message previews after retention: %w", err)
		}
	}
	return affected, nil
}
