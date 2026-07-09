package users

import (
	"context"
	"database/sql"
	"errors"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

const tableExprParentMessagesAsMessage = `users.parent_messages AS "parent_message"`

// ParentMessageRepository is the tenant-scoped data-access layer for messages.
type ParentMessageRepository struct {
	*base.Repository[*users.ParentMessage]
}

// NewParentMessageRepository wires a fresh repository.
func NewParentMessageRepository(db *bun.DB) users.ParentMessageRepository {
	repo := base.NewRepository[*users.ParentMessage](db, "users.parent_messages", "ParentMessage")
	repo.TenantScoped = true
	return &ParentMessageRepository{Repository: repo}
}

// FindByID returns a message row by id within the current tenant, or nil when
// absent. Delegates to the generic base helper so the schema-qualified query
// and tenant filter are not duplicated (backend-conventions Rule 2).
func (r *ParentMessageRepository) FindByID(ctx context.Context, id int64) (*users.ParentMessage, error) {
	return r.FindByIDOrNil(ctx, id)
}

// FindByIDForUpdate is FindByID with a SELECT … FOR UPDATE row lock, so a
// confirm/reject re-reads and locks the request inside the request transaction
// before applying — two staff confirming the same request serialize instead of
// both applying it.
func (r *ParentMessageRepository) FindByIDForUpdate(ctx context.Context, id int64) (*users.ParentMessage, error) {
	message := new(users.ParentMessage)
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(message).
		ModelTableExpr(tableExprParentMessagesAsMessage).
		Where(`"parent_message".id = ?`, id).
		Limit(1).
		For("UPDATE")

	query = base.WithTenantFilter(ctx, query, "parent_message")

	err := query.Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{Op: "find parent message for update", Err: err}
	}
	return message, nil
}

// FindEventByRef returns the system-event message in a thread that references a
// specific record (event_type + ref_table + ref_id), or nil when none exists.
// Used to locate the "request created" pill for a change request so a staff
// decision can advance the deciding admin's read cursor exactly up to it,
// without opening the thread. Oldest match wins (there is at most one).
func (r *ParentMessageRepository) FindEventByRef(ctx context.Context, threadID int64, eventType, refTable string, refID int64) (*users.ParentMessage, error) {
	message := new(users.ParentMessage)
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(message).
		ModelTableExpr(tableExprParentMessagesAsMessage).
		Where(`"parent_message".thread_id = ?`, threadID).
		Where(`"parent_message".kind = ?`, users.ParentMessageKindEvent).
		Where(`"parent_message".event_type = ?`, eventType).
		Where(`"parent_message".ref_table = ?`, refTable).
		Where(`"parent_message".ref_id = ?`, refID).
		OrderExpr(`"parent_message".id ASC`).
		Limit(1)

	query = base.WithTenantFilter(ctx, query, "parent_message")

	err := query.Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{Op: "find parent message event by ref", Err: err}
	}
	return message, nil
}

// ListByThread returns a thread's messages oldest-first (chat order). A positive
// limit returns the most recent `limit` messages (the conversation tail, reversed
// back into chat order); limit <= 0 returns the FULL thread.
//
// limit <= 0 MUST stay a complete snapshot. The read-marking paths
// (messaging.markReadAndBuild and parent.GetChildConversation) advance the
// reader's cursor to the newest message in the returned set, so any message
// omitted from the set would be silently marked read though it was never shown.
// A capped tail is therefore only safe for a future load-older UI (#1672
// follow-up) that pages via a positive limit and never feeds the read-marking
// cursor.
func (r *ParentMessageRepository) ListByThread(ctx context.Context, threadID int64, limit int) ([]*users.ParentMessage, error) {
	var messages []*users.ParentMessage
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&messages).
		ModelTableExpr(tableExprParentMessagesAsMessage).
		Where(`"parent_message".thread_id = ?`, threadID).
		// Newest-first so a positive limit keeps the most recent rows; reversed to
		// oldest-first chat order below.
		OrderExpr(`"parent_message".created_at DESC`).
		OrderExpr(`"parent_message".id DESC`)
	// Only bound the load when an explicit positive limit is requested; limit <= 0
	// returns every message (full snapshot) for the read-marking paths.
	if limit > 0 {
		query = query.Limit(limit)
	}

	query = base.WithTenantFilter(ctx, query, "parent_message")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list parent messages", Err: err}
	}
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	return messages, nil
}
