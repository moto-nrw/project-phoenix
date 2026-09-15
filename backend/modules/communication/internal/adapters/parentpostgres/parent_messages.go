package parentpostgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"
	"github.com/uptrace/bun"
)

const (
	parentMessageTable     = "users.parent_messages"
	parentMessageTableExpr = `users.parent_messages AS "parent_message"`
	parentMessageAlias     = "parent_message"
)

// MessageStore is the append-only log of one child's parent/OGS conversation.
type MessageStore struct{ store }

// NewMessageStore builds the message store over the ambient transaction runtime.
func NewMessageStore(database Database) *MessageStore {
	return &MessageStore{store: newStore(database)}
}

type parentMessageRow struct {
	bun.BaseModel    `bun:"table:users.parent_messages,alias:parent_message"`
	ID               int64          `bun:"id,pk,autoincrement"`
	TenantID         int64          `bun:"tenant_id,notnull"`
	CreatedAt        time.Time      `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt        time.Time      `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	ThreadID         int64          `bun:"thread_id,notnull"`
	StudentID        int64          `bun:"student_id,notnull"`
	SenderAccountID  int64          `bun:"sender_account_id,notnull"`
	SenderKind       string         `bun:"sender_kind,notnull"`
	SenderName       string         `bun:"sender_name,notnull"`
	StaffNameVisible bool           `bun:"staff_name_visible,notnull,default:false"`
	Body             string         `bun:"body,notnull"`
	Kind             string         `bun:"kind,notnull,default:'message'"`
	EventType        string         `bun:"event_type,nullzero"`
	EventActorKind   string         `bun:"event_actor_kind,nullzero"`
	RequestType      string         `bun:"request_type,nullzero"`
	RequestStatus    string         `bun:"request_status,nullzero"`
	Payload          map[string]any `bun:"payload,type:jsonb"`
	RefTable         string         `bun:"ref_table,nullzero"`
	RefID            *int64         `bun:"ref_id"`
	AppliedAt        *time.Time     `bun:"applied_at"`
	AppliedBy        *int64         `bun:"applied_by"`
	DecisionReason   string         `bun:"decision_reason,nullzero"`
}

func (r *parentMessageRow) value() *domain.ParentMessage {
	if r == nil {
		return nil
	}
	return &domain.ParentMessage{
		ID: r.ID, TenantID: r.TenantID, ThreadID: r.ThreadID, StudentID: r.StudentID,
		SenderAccountID: r.SenderAccountID, SenderKind: r.SenderKind, SenderName: r.SenderName,
		StaffNameVisible: r.StaffNameVisible, Body: r.Body, Kind: r.Kind,
		EventType: r.EventType, EventActorKind: r.EventActorKind,
		RequestType: r.RequestType, RequestStatus: r.RequestStatus, Payload: r.Payload,
		RefTable: r.RefTable, RefID: r.RefID, AppliedAt: r.AppliedAt, AppliedBy: r.AppliedBy,
		DecisionReason: r.DecisionReason, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func messageRow(value *domain.ParentMessage) *parentMessageRow {
	return &parentMessageRow{
		ID: value.ID, TenantID: value.TenantID, ThreadID: value.ThreadID, StudentID: value.StudentID,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		SenderAccountID: value.SenderAccountID, SenderKind: value.SenderKind, SenderName: value.SenderName,
		StaffNameVisible: value.StaffNameVisible, Body: value.Body, Kind: value.Kind,
		EventType: value.EventType, EventActorKind: value.EventActorKind,
		RequestType: value.RequestType, RequestStatus: value.RequestStatus, Payload: value.Payload,
		RefTable: value.RefTable, RefID: value.RefID, AppliedAt: value.AppliedAt, AppliedBy: value.AppliedBy,
		DecisionReason: value.DecisionReason,
	}
}

// Create appends one message. The database stamps created_at, and the caller
// reads it back off the returned value: the thread preview is advanced from the
// row's own clock, never the application's.
func (s *MessageStore) Create(ctx context.Context, message *domain.ParentMessage) error {
	if message == nil {
		return errors.New("create parent message: message is required")
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	if message.TenantID == 0 {
		message.TenantID = tenantID
	}
	row := messageRow(message)
	if _, err := db.NewInsert().
		Model(row).
		ModelTableExpr(parentMessageTable).
		Returning("id, created_at, updated_at").
		Exec(ctx); err != nil {
		return fmt.Errorf("create parent message: %w", err)
	}
	message.ID = row.ID
	message.CreatedAt = row.CreatedAt
	message.UpdatedAt = row.UpdatedAt
	return nil
}

// Update rewrites one message row by primary key. Only the decision fields of a
// request message ever change; the body is append-only.
func (s *MessageStore) Update(ctx context.Context, message *domain.ParentMessage) error {
	if message == nil || message.ID <= 0 {
		return errors.New("update parent message: message id is required")
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	query := db.NewUpdate().
		Model(messageRow(message)).
		ModelTableExpr(parentMessageTableExpr).
		WherePK()
	query = withTenant(query, parentMessageAlias, tenantID)
	result, err := query.Exec(ctx)
	if err != nil {
		return fmt.Errorf("update parent message: %w", err)
	}
	return assertOneRow(result, "update parent message")
}

// FindByID returns the message within the current tenant, or nil when absent.
func (s *MessageStore) FindByID(ctx context.Context, id int64) (*domain.ParentMessage, error) {
	return s.findMessage(ctx, id, false)
}

// FindByIDForUpdate is FindByID with a row lock, so two staff confirming the
// same request serialize instead of both applying it.
func (s *MessageStore) FindByIDForUpdate(ctx context.Context, id int64) (*domain.ParentMessage, error) {
	return s.findMessage(ctx, id, true)
}

func (s *MessageStore) findMessage(ctx context.Context, id int64, lock bool) (*domain.ParentMessage, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	row := new(parentMessageRow)
	query := db.NewSelect().
		Model(row).
		ModelTableExpr(parentMessageTableExpr).
		Where(`"parent_message".id = ?`, id).
		Limit(1)
	if lock {
		query = query.For("UPDATE")
	}
	query = withTenant(query, parentMessageAlias, tenantID)
	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find parent message: %w", err)
	}
	return row.value(), nil
}

// FindEventByRef returns the system-event message in a thread that references a
// specific record, or nil when none exists. It locates a change request's
// "request created" pill so a decision can mark it read without opening the
// thread. Oldest match wins; there is at most one.
func (s *MessageStore) FindEventByRef(ctx context.Context, threadID int64, eventType, refTable string, refID int64) (*domain.ParentMessage, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	row := new(parentMessageRow)
	query := db.NewSelect().
		Model(row).
		ModelTableExpr(parentMessageTableExpr).
		Where(`"parent_message".thread_id = ?`, threadID).
		Where(`"parent_message".kind = ?`, "event").
		Where(`"parent_message".event_type = ?`, eventType).
		Where(`"parent_message".ref_table = ?`, refTable).
		Where(`"parent_message".ref_id = ?`, refID).
		OrderExpr(`"parent_message".id ASC`).
		Limit(1)
	query = withTenant(query, parentMessageAlias, tenantID)
	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find parent message event by ref: %w", err)
	}
	return row.value(), nil
}

// ListByThread returns a thread's messages oldest-first (chat order). A
// positive limit returns the most recent limit messages; limit <= 0 returns the
// FULL thread.
//
// limit <= 0 MUST stay a complete snapshot: the read-marking paths advance the
// reader's cursor to the newest message in the returned set, so any omitted
// message would be silently marked read though it was never shown. A capped
// tail is only safe for a load-older UI that never feeds the read cursor.
func (s *MessageStore) ListByThread(ctx context.Context, threadID int64, limit int) ([]*domain.ParentMessage, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []parentMessageRow
	query := db.NewSelect().
		Model(&rows).
		ModelTableExpr(parentMessageTableExpr).
		Where(`"parent_message".thread_id = ?`, threadID).
		// Newest-first so a positive limit keeps the most recent rows; reversed
		// to oldest-first chat order below.
		OrderExpr(`"parent_message".created_at DESC`).
		OrderExpr(`"parent_message".id DESC`)
	if limit > 0 {
		query = query.Limit(limit)
	}
	query = withTenant(query, parentMessageAlias, tenantID)
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("list parent messages: %w", err)
	}
	messages := make([]*domain.ParentMessage, len(rows))
	for i := range rows {
		messages[len(rows)-1-i] = rows[i].value()
	}
	return messages, nil
}
